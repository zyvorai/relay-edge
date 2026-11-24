// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

// Package replay re-publishes a recorded or hand-written event stream with its
// original relative pacing scaled by a speed factor. Idempotency keys are a
// pure function of the run id and position, so replaying the same stream with
// the same run id is a no-op for a Relay that deduplicates correctly.
package replay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"sync"
	"time"

	"github.com/zyvorai/relay-edge/internal/journal"
)

// Limits.
const (
	MaxEvents      = 10_000
	MinSpeed       = 0.01
	MaxSpeed       = 1000
	MaxRepeat      = 100
	MaxTotalSteps  = 100_000 // events x repeat
	DefaultMaxGap  = 5 * time.Second
	HardMaxGap     = 30 * time.Second
	maxRunIDLength = 64

	MinRate       = 0.01
	MaxRate       = 1000.0 // events per second
	MaxStages     = 20
	MaxStageMS    = 3_600_000
	MaxShuffleWin = 1000
)

// Stage is one leg of a rate profile: the publish rate moves linearly from the
// previous leg's rate to RatePerSec over DurationMS (k6 ramping-arrival-rate
// semantics). Equal rates hold steady.
type Stage struct {
	DurationMS int64   `json:"duration_ms"`
	RatePerSec float64 `json:"rate_per_sec"`
}

// Options controls one run.
type Options struct {
	Speed  float64       // 1 = recorded pacing; 0 means 1
	MaxGap time.Duration // cap on any single pause (after speed scaling); 0 = DefaultMaxGap
	RunID  string        // key namespace; "" = content hash of the stream
	DryRun bool          // plan only, publish nothing
	Repeat int           // play the stream this many times back to back; 0 = 1

	// Open-model pacing: ignore recorded timestamps and publish at a target
	// rate. RatePerSec is a constant rate; Stages is a ramp profile (the last
	// rate is held once the stages end); StartRate is the rate at t=0 for a
	// profile (default: the first stage's rate). Exclusive with Speed.
	RatePerSec float64
	StartRate  float64
	Stages     []Stage

	// ShuffleWindow (2..1000) permutes events within consecutive windows,
	// seeded by ShuffleSeed, to test Relay's handling of out-of-order arrival.
	// Pacing and each event's idempotency key (by original position) are kept.
	ShuffleWindow int
	ShuffleSeed   int64
}

func (o Options) rateMode() bool { return o.RatePerSec != 0 || len(o.Stages) > 0 }

// Step is one planned publish.
type Step struct {
	Index int           `json:"index"`
	Delay time.Duration `json:"delay_ns"`
	Key   string        `json:"idempotency_key"`
	Entry journal.Entry `json:"event"`
}

// Normalize validates and fills defaults.
func (o Options) Normalize() (Options, error) {
	if o.rateMode() && o.Speed != 0 {
		return o, errors.New("speed cannot be combined with rate_per_sec or rate_stages")
	}
	if o.Speed == 0 {
		o.Speed = 1
	}
	if o.Speed != o.Speed || o.Speed < MinSpeed || o.Speed > MaxSpeed {
		return o, fmt.Errorf("speed must be between %v and %v", MinSpeed, MaxSpeed)
	}
	if o.rateMode() {
		if err := o.validateRate(); err != nil {
			return o, err
		}
	}
	if o.ShuffleWindow != 0 && (o.ShuffleWindow < 2 || o.ShuffleWindow > MaxShuffleWin) {
		return o, fmt.Errorf("shuffle_window must be 0 or between 2 and %d", MaxShuffleWin)
	}
	if o.Repeat == 0 {
		o.Repeat = 1
	}
	if o.Repeat < 1 || o.Repeat > MaxRepeat {
		return o, fmt.Errorf("repeat must be between 1 and %d", MaxRepeat)
	}
	if o.MaxGap == 0 {
		o.MaxGap = DefaultMaxGap
	}
	if o.MaxGap < 0 || o.MaxGap > HardMaxGap {
		return o, fmt.Errorf("max_gap_ms must be between 0 and %d", HardMaxGap.Milliseconds())
	}
	if len(o.RunID) > maxRunIDLength {
		return o, errors.New("run_id too long")
	}
	for _, r := range o.RunID {
		if !isRunIDRune(r) {
			return o, errors.New("run_id may contain only letters, digits, '-', '_' and '.'")
		}
	}
	return o, nil
}

func okRate(r float64) bool { return r == r && r >= MinRate && r <= MaxRate }

func (o Options) validateRate() error {
	if o.RatePerSec != 0 && len(o.Stages) > 0 {
		return errors.New("use rate_per_sec or rate_stages, not both")
	}
	if o.RatePerSec != 0 && !okRate(o.RatePerSec) {
		return fmt.Errorf("rate_per_sec must be between %v and %v", MinRate, MaxRate)
	}
	if o.StartRate != 0 && !okRate(o.StartRate) {
		return fmt.Errorf("start_rate must be between %v and %v", MinRate, MaxRate)
	}
	if len(o.Stages) > MaxStages {
		return fmt.Errorf("at most %d rate_stages", MaxStages)
	}
	for i, st := range o.Stages {
		if st.DurationMS < 1 || st.DurationMS > MaxStageMS {
			return fmt.Errorf("rate_stages[%d].duration_ms must be between 1 and %d", i, MaxStageMS)
		}
		if !okRate(st.RatePerSec) {
			return fmt.Errorf("rate_stages[%d].rate_per_sec must be between %v and %v", i, MinRate, MaxRate)
		}
	}
	return nil
}

// rateAt returns the target rate (events/s) t seconds into the profile.
func (o Options) rateAt(t float64) float64 {
	if o.RatePerSec != 0 {
		return o.RatePerSec
	}
	prev := o.StartRate
	if prev == 0 {
		prev = o.Stages[0].RatePerSec
	}
	start := 0.0
	for _, st := range o.Stages {
		dur := float64(st.DurationMS) / 1000
		if t < start+dur {
			return prev + (st.RatePerSec-prev)*(t-start)/dur
		}
		start += dur
		prev = st.RatePerSec
	}
	return prev
}

// order returns the playback permutation for one pass: identity, or a seeded
// shuffle inside consecutive windows.
func (o Options) order(n, loop int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	if o.ShuffleWindow < 2 {
		return idx
	}
	rng := rand.New(rand.NewSource(o.ShuffleSeed + int64(loop)))
	for lo := 0; lo < n; lo += o.ShuffleWindow {
		w := idx[lo:min(lo+o.ShuffleWindow, n)]
		rng.Shuffle(len(w), func(a, b int) { w[a], w[b] = w[b], w[a] })
	}
	return idx
}

// ContentID derives a stable run id from the stream content.
func ContentID(events []journal.Entry) string {
	h := sha256.New()
	for _, e := range events {
		b, _ := json.Marshal([]any{e.Type, e.Severity, e.Source, e.Key, e.Data})
		h.Write(b)
		h.Write([]byte{'\n'})
	}
	return "c" + hex.EncodeToString(h.Sum(nil))[:12]
}

// Plan computes the publish schedule. It is pure: same input, same output.
func Plan(events []journal.Entry, o Options) ([]Step, Options, error) {
	o, err := o.Normalize()
	if err != nil {
		return nil, o, err
	}
	if len(events) == 0 {
		return nil, o, errors.New("no events")
	}
	if len(events) > MaxEvents {
		return nil, o, fmt.Errorf("at most %d events per run", MaxEvents)
	}
	if o.RunID == "" {
		o.RunID = ContentID(events)
	}
	if len(events)*o.Repeat > MaxTotalSteps {
		return nil, o, fmt.Errorf("events x repeat must not exceed %d", MaxTotalSteps)
	}
	steps := make([]Step, 0, len(events)*o.Repeat)
	clock := 0.0 // seconds on the rate profile, running across passes
	for loop := 1; loop <= o.Repeat; loop++ {
		order := o.order(len(events), loop)
		for p := range events {
			i := order[p] // original position of the event played at slot p
			e := events[i]
			var d time.Duration
			switch {
			case o.rateMode():
				if len(steps) > 0 {
					gap := 1 / o.rateAt(clock)
					clock += gap
					d = min(time.Duration(gap*float64(time.Second)), o.MaxGap)
				}
			case p > 0 && !events[p].At.IsZero() && !events[p-1].At.IsZero():
				if gap := events[p].At.Sub(events[p-1].At); gap > 0 {
					d = min(time.Duration(float64(gap)/o.Speed), o.MaxGap)
				}
			}
			// Pass 1 keeps the original key shape; later passes get their own
			// namespace so each pass is a distinct, repeatable set of events.
			key := fmt.Sprintf("replay/%s/%d", o.RunID, i+1)
			if loop > 1 {
				key = fmt.Sprintf("replay/%s/p%d/%d", o.RunID, loop, i+1)
			}
			steps = append(steps, Step{Index: (loop-1)*len(events) + p + 1, Delay: d, Key: key, Entry: e})
		}
	}
	return steps, o, nil
}

// Publish sends one step; return an error to count it as failed.
type Publish func(Step) error

// Status is a snapshot of the current or last run.
type Status struct {
	RunID      string     `json:"run_id"`
	State      string     `json:"state"` // idle | running | done | cancelled
	Total      int        `json:"total"`
	Sent       int        `json:"sent"`
	Failed     int        `json:"failed"`
	Speed      float64    `json:"speed"`
	Repeat     int        `json:"repeat"`
	Pacing     string     `json:"pacing,omitempty"`     // recorded | rate
	Latency    *Latency   `json:"latency_ms,omitempty"` // publish call latency, set once any event was attempted
	RatePerSec float64    `json:"rate_per_sec,omitempty"`
	DryRun     bool       `json:"dry_run"`
	LastError  string     `json:"last_error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// Latency summarizes publish call durations in milliseconds.
type Latency struct {
	Min float64 `json:"min"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	Max float64 `json:"max"`
}

// Summarize computes nearest-rank percentiles; nil for no samples.
func Summarize(d []time.Duration) *Latency {
	if len(d) == 0 {
		return nil
	}
	s := append([]time.Duration(nil), d...)
	slices.Sort(s)
	ms := func(x time.Duration) float64 { return float64(x.Microseconds()) / 1000 }
	rank := func(p float64) time.Duration {
		i := int(math.Ceil(p*float64(len(s)))) - 1
		return s[min(max(i, 0), len(s)-1)]
	}
	return &Latency{Min: ms(s[0]), P50: ms(rank(0.50)), P95: ms(rank(0.95)), Max: ms(s[len(s)-1])}
}

// ErrBusy is returned by Start while a run is active.
var ErrBusy = errors.New("replay: a run is already in progress")

// Runner executes one replay at a time.
type Runner struct {
	mu     sync.Mutex
	st     Status
	cancel context.CancelFunc
	done   chan struct{}
	// Sleep waits d or until ctx ends; replaceable in tests.
	Sleep func(ctx context.Context, d time.Duration) error
}

// NewRunner returns an idle runner using real sleeps.
func NewRunner() *Runner { return &Runner{st: Status{State: "idle"}, Sleep: realSleep} }

func realSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Status returns a copy of the current status.
func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.st
}

// Start plans and launches a run in the background. A dry run completes
// immediately without publishing or sleeping.
func (r *Runner) Start(events []journal.Entry, o Options, pub Publish) (Status, []Step, error) {
	steps, o, err := Plan(events, o)
	if err != nil {
		return Status{}, nil, err
	}
	r.mu.Lock()
	if r.st.State == "running" {
		r.mu.Unlock()
		return r.Status(), nil, ErrBusy
	}
	now := time.Now().UTC()
	r.st = Status{RunID: o.RunID, State: "running", Total: len(steps), Speed: o.Speed, Repeat: o.Repeat, Pacing: pacing(o), DryRun: o.DryRun, StartedAt: &now}
	if o.DryRun {
		fin := now
		r.st.State, r.st.FinishedAt = "done", &fin
		st := r.st
		r.mu.Unlock()
		return st, steps, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	done := make(chan struct{})
	r.done = done
	st := r.st
	r.mu.Unlock()
	go r.run(ctx, steps, pub, done)
	return st, steps, nil
}

func pacing(o Options) string {
	if o.rateMode() {
		return "rate"
	}
	return "recorded"
}

func (r *Runner) run(ctx context.Context, steps []Step, pub Publish, done chan struct{}) {
	defer close(done)
	final := "done"
	var lat []time.Duration
	began := time.Now()
	for _, s := range steps {
		if err := r.Sleep(ctx, s.Delay); err != nil {
			final = "cancelled"
			break
		}
		t0 := time.Now()
		err := pub(s)
		lat = append(lat, time.Since(t0))
		r.mu.Lock()
		if err != nil {
			r.st.Failed++
			r.st.LastError = err.Error()
		} else {
			r.st.Sent++
		}
		r.mu.Unlock()
	}
	r.mu.Lock()
	fin := time.Now().UTC()
	r.st.Latency = Summarize(lat)
	if el := time.Since(began).Seconds(); el > 0 && len(lat) > 0 {
		r.st.RatePerSec = math.Round(float64(len(lat))/el*100) / 100
	}
	r.st.State, r.st.FinishedAt = final, &fin
	r.mu.Unlock()
}

// Cancel stops an active run; it reports whether one was running.
func (r *Runner) Cancel() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.st.State != "running" || r.cancel == nil {
		return false
	}
	r.cancel()
	return true
}

// Wait blocks until the current run (if any) has finished. Used by tests and
// graceful shutdown.
func (r *Runner) Wait() {
	r.mu.Lock()
	d := r.done
	r.mu.Unlock()
	if d != nil {
		<-d
	}
}

func isRunIDRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
}
