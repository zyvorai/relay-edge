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
	"sync"
	"time"

	"github.com/zyvorai/relay-edge/internal/journal"
)

// Limits.
const (
	MaxEvents      = 10_000
	MinSpeed       = 0.01
	MaxSpeed       = 1000
	DefaultMaxGap  = 5 * time.Second
	HardMaxGap     = 30 * time.Second
	maxRunIDLength = 64
)

// Options controls one run.
type Options struct {
	Speed  float64       // 1 = recorded pacing; 0 means 1
	MaxGap time.Duration // cap on any single pause (after speed scaling); 0 = DefaultMaxGap
	RunID  string        // key namespace; "" = content hash of the stream
	DryRun bool          // plan only, publish nothing
}

// Step is one planned publish.
type Step struct {
	Index int           `json:"index"`
	Delay time.Duration `json:"delay_ns"`
	Key   string        `json:"idempotency_key"`
	Entry journal.Entry `json:"event"`
}

// Normalize validates and fills defaults.
func (o Options) Normalize() (Options, error) {
	if o.Speed == 0 {
		o.Speed = 1
	}
	if o.Speed != o.Speed || o.Speed < MinSpeed || o.Speed > MaxSpeed {
		return o, fmt.Errorf("speed must be between %v and %v", MinSpeed, MaxSpeed)
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
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return o, errors.New("run_id may contain only letters, digits, '-', '_' and '.'")
		}
	}
	return o, nil
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
	steps := make([]Step, 0, len(events))
	for i, e := range events {
		var d time.Duration
		if i > 0 && !e.At.IsZero() && !events[i-1].At.IsZero() {
			if gap := e.At.Sub(events[i-1].At); gap > 0 {
				d = time.Duration(float64(gap) / o.Speed)
				if d > o.MaxGap {
					d = o.MaxGap
				}
			}
		}
		steps = append(steps, Step{Index: i + 1, Delay: d, Key: fmt.Sprintf("replay/%s/%d", o.RunID, i+1), Entry: e})
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
	DryRun     bool       `json:"dry_run"`
	LastError  string     `json:"last_error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
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
	r.st = Status{RunID: o.RunID, State: "running", Total: len(steps), Speed: o.Speed, DryRun: o.DryRun, StartedAt: &now}
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

func (r *Runner) run(ctx context.Context, steps []Step, pub Publish, done chan struct{}) {
	defer close(done)
	final := "done"
	for _, s := range steps {
		if err := r.Sleep(ctx, s.Delay); err != nil {
			final = "cancelled"
			break
		}
		err := pub(s)
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
