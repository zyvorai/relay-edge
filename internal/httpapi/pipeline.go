// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zyvorai/relay-edge/internal/chaos"
	"github.com/zyvorai/relay-edge/internal/journal"
	"github.com/zyvorai/relay-edge/internal/replay"
)

// maxPendingDelayed bounds goroutines/timers held by chaos delay.
const maxPendingDelayed = 1000

// pipeline groups the deterministic-simulation, journal, replay and chaos state.
type pipeline struct {
	Journal *journal.Journal
	Chaos   *chaos.Injector
	Replay  *replay.Runner

	seeded  atomic.Bool
	seed    atomic.Int64
	keySeq  atomic.Int64
	pending atomic.Int64
}

func newPipeline(journalCap int) *pipeline {
	return &pipeline{Journal: journal.New(journalCap), Chaos: chaos.New(), Replay: replay.NewRunner()}
}

// SetSimSeed makes the simulators deterministic. A nil seed restores the
// default (time-seeded noise, timestamp-based idempotency keys).
func (s *Server) SetSimSeed(seed *int64) {
	p := s.pipe
	if seed == nil {
		p.seeded.Store(false)
		p.keySeq.Store(0)
		if s.FW != nil {
			s.FW.SetSeed(time.Now().UnixNano())
		}
		if s.RemoteEdge != nil {
			s.RemoteEdge.SetSeed(time.Now().UnixNano() + 1)
		}
		if s.Fleet != nil {
			s.Fleet.SetSeed(time.Now().UnixNano() + 2)
		}
		return
	}
	p.seed.Store(*seed)
	p.keySeq.Store(0)
	p.seeded.Store(true)
	if s.FW != nil {
		s.FW.SetSeed(*seed)
	}
	if s.RemoteEdge != nil {
		s.RemoteEdge.SetSeed(*seed + 1)
	}
	if s.Fleet != nil {
		s.Fleet.SetSeed(*seed + 2)
	}
}

// simKey builds an idempotency key. With a seed set it is a pure function of
// (prefix, seed, call order); otherwise it embeds the wall clock as before.
func (s *Server) simKey(prefix string) string {
	if s.pipe.seeded.Load() {
		return fmt.Sprintf("%s/s%d-%d", prefix, s.pipe.seed.Load(), s.pipe.keySeq.Add(1))
	}
	return fmt.Sprintf("%s/%d", prefix, time.Now().UnixNano())
}

// emit journals an event and hands it to the publish path through the chaos
// injector. It is the single choke point for simulator and replay publishes.
func (s *Server) emit(origin, evType, severity, source, key string, data map[string]any) error {
	_, err := s.emitSeq(origin, evType, severity, source, key, data)
	return err
}

// emitSeq is emit that also returns the journal sequence number assigned.
func (s *Server) emitSeq(origin, evType, severity, source, key string, data map[string]any) (int64, error) {
	p := s.pipe
	e := p.Journal.Add(journal.Entry{Type: evType, Severity: severity, Source: source, Key: key, Data: data, Origin: origin})
	d := p.Chaos.DecideFor(evType)
	if s.metrics != nil && d.Action != "" {
		s.metrics.chaos.WithLabelValues(string(d.Action)).Inc()
	}
	send := func() error {
		_, err := s.Pub.PublishEventType(evType, severity, source, key, data)
		return err
	}
	switch d.Action {
	case chaos.Drop:
		p.Journal.SetOutcome(e.Seq, "dropped")
		return e.Seq, nil
	case chaos.Error:
		p.Journal.SetOutcome(e.Seq, "error")
		return e.Seq, chaos.ErrInjected
	}
	run := func() error {
		err := send()
		outcome := "published"
		if err == nil && d.Action == chaos.Duplicate {
			err = send()
			outcome = "duplicated"
		}
		if err != nil {
			outcome = "error"
		}
		p.Journal.SetOutcome(e.Seq, outcome)
		return err
	}
	if d.Delay > 0 && p.pending.Add(1) <= maxPendingDelayed {
		time.AfterFunc(d.Delay, func() {
			defer p.pending.Add(-1)
			if err := run(); err != nil {
				slog.Error("publish (delayed)", "type", evType, "error", err)
			}
		})
		return e.Seq, nil
	} else if d.Delay > 0 {
		p.pending.Add(-1)
	}
	return e.Seq, run()
}

func (s *Server) mountPipeline() {
	s.Mux.HandleFunc("GET /v1/sim/seed", s.getSimSeed)
	s.Mux.HandleFunc("POST /v1/sim/seed", s.postSimSeed)
	s.Mux.HandleFunc("GET /v1/journal", s.listJournal)
	s.Mux.HandleFunc("GET /v1/journal/export", s.exportJournal)
	s.Mux.HandleFunc("POST /v1/journal/clear", s.clearJournal)
	s.Mux.HandleFunc("POST /v1/journal/diff", s.diffJournal)
	s.Mux.HandleFunc("GET /v1/journal/stats", s.journalStats)
	s.Mux.HandleFunc("POST /v1/journal/resend", s.resendJournal)
	s.Mux.HandleFunc("GET /v1/chaos", s.getChaos)
	s.Mux.HandleFunc("POST /v1/chaos/config", s.postChaos)
	s.Mux.HandleFunc("POST /v1/chaos/reset", s.resetChaos)
	s.Mux.HandleFunc("GET /v1/replay", s.getReplay)
	s.Mux.HandleFunc("POST /v1/replay", s.postReplay)
	s.Mux.HandleFunc("POST /v1/replay/cancel", s.cancelReplay)
}

func (s *Server) simSeedState() map[string]any {
	m := map[string]any{"seeded": s.pipe.seeded.Load()}
	if s.pipe.seeded.Load() {
		m["seed"] = s.pipe.seed.Load()
	}
	return m
}

func (s *Server) getSimSeed(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.simSeedState())
}

func (s *Server) postSimSeed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Seed *int64 `json:"seed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.SetSimSeed(body.Seed)
	writeJSON(w, 200, s.simSeedState())
}

func queryInt(r *http.Request, name string, def int64) (int64, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return n, nil
}

func filterOrigin(in []journal.Entry, origin string) []journal.Entry {
	if origin == "" {
		return in
	}
	out := in[:0:0]
	for _, e := range in {
		if e.Origin == origin {
			out = append(out, e)
		}
	}
	return out
}

func (s *Server) listJournal(w http.ResponseWriter, r *http.Request) {
	after, err := queryInt(r, "after", 0)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	limit, err := queryInt(r, "limit", 100)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	items := filterOrigin(s.pipe.Journal.List(after, int(min(limit, journal.MaxListPageLimit))), r.URL.Query().Get("origin"))
	writeJSON(w, 200, map[string]any{"items": items, "count": len(items), "retained": s.pipe.Journal.Len(), "capacity": s.pipe.Journal.Cap()})
}

func (s *Server) exportJournal(w http.ResponseWriter, r *http.Request) {
	from, err := queryInt(r, "from", 0)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	to, err := queryInt(r, "to", 0)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	limit, err := queryInt(r, "limit", replay.MaxEvents)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	limit = min(max(limit, 1), replay.MaxEvents)
	items := filterOrigin(s.pipe.Journal.Range(from, to, int(limit)), r.URL.Query().Get("origin"))
	switch f := r.URL.Query().Get("format"); f {
	case "", "ndjson":
		w.Header().Set("Content-Type", "application/x-ndjson")
		_ = journal.WriteNDJSON(w, items)
	case "cloudevents":
		w.Header().Set("Content-Type", "application/cloudevents-batch+json")
		_ = json.NewEncoder(w).Encode(journal.ToCloudEvents(items))
	default:
		writeJSON(w, 400, map[string]any{"error": "format must be ndjson or cloudevents"})
	}
}

func (s *Server) clearJournal(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"cleared": s.pipe.Journal.Clear()})
}

func (s *Server) chaosState() map[string]any {
	cfg, st := s.pipe.Chaos.State()
	return map[string]any{"config": cfg, "stats": st, "pending_delayed": s.pipe.pending.Load()}
}

func (s *Server) getChaos(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.chaosState())
}

func (s *Server) postChaos(w http.ResponseWriter, r *http.Request) {
	var cfg chaos.Config
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if err := s.pipe.Chaos.Configure(cfg); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, s.chaosState())
}

func (s *Server) resetChaos(w http.ResponseWriter, _ *http.Request) {
	s.pipe.Chaos.Reset()
	writeJSON(w, 200, s.chaosState())
}

func (s *Server) getReplay(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.pipe.Replay.Status())
}

func (s *Server) cancelReplay(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"cancelled": s.pipe.Replay.Cancel()})
}

type replayBody struct {
	Events      json.RawMessage `json:"events"`
	FromJournal bool            `json:"from_journal"`
	FromSeq     int64           `json:"from_seq"`
	ToSeq       int64           `json:"to_seq"`
	Origin      string          `json:"origin"`
	Speed       float64         `json:"speed"`
	MaxGapMS    int64           `json:"max_gap_ms"`
	RunID       string          `json:"run_id"`
	DryRun      bool            `json:"dry_run"`
	MaxEvents   int             `json:"max_events"`
	Types       []string        `json:"types"`      // keep only these event types; trailing * is a prefix
	Severities  []string        `json:"severities"` // keep only these severities
	Repeat      int             `json:"repeat"`     // play the stream N times (1..100)

	RatePerSec    float64        `json:"rate_per_sec"`   // open-model constant rate, ignores recorded timestamps
	StartRate     float64        `json:"start_rate"`     // rate at t=0 for rate_stages
	RateStages    []replay.Stage `json:"rate_stages"`    // ramp profile
	ShuffleWindow int            `json:"shuffle_window"` // permute events inside windows of N (2..1000)
	ShuffleSeed   int64          `json:"shuffle_seed"`
}

func (s *Server) postReplay(w http.ResponseWriter, r *http.Request) {
	var b replayBody
	var raw []byte
	ct := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if ct == "application/x-ndjson" || ct == "application/cloudevents-batch+json" || ct == "application/cloudevents+json" {
		var err error
		raw, err = io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		q := r.URL.Query()
		b.Speed, _ = strconv.ParseFloat(q.Get("speed"), 64)
		b.MaxGapMS, _ = strconv.ParseInt(q.Get("max_gap_ms"), 10, 64)
		b.RunID = q.Get("run_id")
		b.DryRun = q.Get("dry_run") == "true" || q.Get("dry_run") == "1"
		b.Repeat, _ = strconv.Atoi(q.Get("repeat"))
		b.Types, b.Severities = splitCSV(q.Get("types")), splitCSV(q.Get("severities"))
		b.RatePerSec, _ = strconv.ParseFloat(q.Get("rate_per_sec"), 64)
		b.StartRate, _ = strconv.ParseFloat(q.Get("start_rate"), 64)
		b.ShuffleWindow, _ = strconv.Atoi(q.Get("shuffle_window"))
		b.ShuffleSeed, _ = strconv.ParseInt(q.Get("shuffle_seed"), 10, 64)
	} else if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	} else {
		raw = b.Events
	}
	maxEv := b.MaxEvents
	if maxEv <= 0 || maxEv > replay.MaxEvents {
		maxEv = replay.MaxEvents
	}
	var events []journal.Entry
	switch {
	case b.FromJournal:
		events = filterOrigin(s.pipe.Journal.Range(b.FromSeq, b.ToSeq, maxEv+1), b.Origin)
		if len(events) > maxEv {
			writeJSON(w, 400, map[string]any{"error": journal.ErrTooMany.Error(), "max_events": maxEv})
			return
		}
	case len(raw) > 0:
		var err error
		events, err = journal.Parse(raw, maxEv)
		if err != nil {
			code := 400
			if errors.Is(err, journal.ErrTooMany) {
				code = 413
			}
			writeJSON(w, code, map[string]any{"error": err.Error(), "max_events": maxEv})
			return
		}
	default:
		writeJSON(w, 400, map[string]any{"error": "supply events or from_journal"})
		return
	}
	events = journal.Filter(events, b.Types, b.Severities)
	if len(events) == 0 {
		writeJSON(w, 400, map[string]any{"error": "no events selected"})
		return
	}
	opts := replay.Options{Repeat: b.Repeat, Speed: b.Speed, MaxGap: time.Duration(b.MaxGapMS) * time.Millisecond, RunID: b.RunID, DryRun: b.DryRun,
		RatePerSec: b.RatePerSec, StartRate: b.StartRate, Stages: b.RateStages, ShuffleWindow: b.ShuffleWindow, ShuffleSeed: b.ShuffleSeed}
	st, steps, err := s.pipe.Replay.Start(events, opts, func(step replay.Step) error {
		s.IncPublish()
		return s.emit("replay", step.Entry.Type, step.Entry.Severity, step.Entry.Source, step.Key, step.Entry.Data)
	})
	switch {
	case errors.Is(err, replay.ErrBusy):
		writeJSON(w, 409, map[string]any{"error": err.Error(), "status": st})
		return
	case err != nil:
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	resp := map[string]any{"status": st}
	if b.DryRun {
		plan := steps
		if len(plan) > 100 {
			plan = plan[:100]
		}
		resp["plan"] = plan
		resp["plan_truncated"] = len(steps) > len(plan)
		writeJSON(w, 200, resp)
		return
	}
	writeJSON(w, 202, resp)
}

func splitCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

type diffBody struct {
	// Expected is the reference stream: any format journal.Parse accepts, or a
	// JSON string holding NDJSON (as downloaded from /v1/journal/export).
	Expected json.RawMessage `json:"expected"`
	// Actual is either an inline stream or, when omitted, a slice of the
	// live journal selected by from_seq/to_seq/origin.
	Actual     json.RawMessage `json:"actual"`
	FromSeq    int64           `json:"from_seq"`
	ToSeq      int64           `json:"to_seq"`
	Origin     string          `json:"origin"`
	IncludeKey bool            `json:"include_key"`
}

// diffJournal compares a reference stream with another stream or with a slice
// of the journal: the regression check for "did this run publish the same
// events as the baseline". Read-only; nothing is published.
func (s *Server) diffJournal(w http.ResponseWriter, r *http.Request) {
	var b diffBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if len(b.Expected) == 0 {
		writeJSON(w, 400, map[string]any{"error": "supply expected"})
		return
	}
	exp, err := journal.Parse(unwrapString(b.Expected), replay.MaxEvents)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "expected: " + err.Error()})
		return
	}
	var act []journal.Entry
	if len(b.Actual) > 0 {
		if act, err = journal.Parse(unwrapString(b.Actual), replay.MaxEvents); err != nil {
			writeJSON(w, 400, map[string]any{"error": "actual: " + err.Error()})
			return
		}
	} else {
		act = filterOrigin(s.pipe.Journal.Range(b.FromSeq, b.ToSeq, replay.MaxEvents), b.Origin)
	}
	writeJSON(w, 200, journal.Diff(exp, act, journal.DiffOptions{IncludeKey: b.IncludeKey}))
}

// unwrapString turns a JSON string value into its contents so NDJSON text can
// be embedded in a JSON body; any other value is returned unchanged.
func unwrapString(raw json.RawMessage) []byte {
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return []byte(str)
	}
	return raw
}

func (s *Server) journalStats(w http.ResponseWriter, r *http.Request) {
	after, err := queryInt(r, "after", 0)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	items := s.pipe.Journal.Range(after+1, 0, 0)
	items = filterOrigin(items, r.URL.Query().Get("origin"))
	items = journal.Filter(items, splitCSV(r.URL.Query().Get("types")), splitCSV(r.URL.Query().Get("severities")))
	writeJSON(w, 200, map[string]any{"stats": journal.Summarize(items), "retained": s.pipe.Journal.Len(), "capacity": s.pipe.Journal.Cap()})
}

const maxResend = 100

type resendBody struct {
	Seq      int64          `json:"seq"`
	Count    int            `json:"count"`
	Key      string         `json:"idempotency_key"` // explicit key; with count>1 gets a -n suffix
	Type     string         `json:"type"`
	Severity string         `json:"severity"`
	Source   string         `json:"source"`
	Set      map[string]any `json:"set"`    // dotted data paths to write
	Remove   []string       `json:"remove"` // dotted data paths to delete
}

// resendJournal re-publishes one retained journal event, optionally edited
// (Stripe CLI resend/trigger style overrides), through the same chaos-aware
// publish path. It publishes only; the new events are journaled with origin
// "resend".
func (s *Server) resendJournal(w http.ResponseWriter, r *http.Request) {
	var b resendBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if b.Count == 0 {
		b.Count = 1
	}
	if b.Count < 1 || b.Count > maxResend {
		writeJSON(w, 400, map[string]any{"error": fmt.Sprintf("count must be between 1 and %d", maxResend)})
		return
	}
	if b.Type != "" && !journal.ValidType(b.Type) {
		writeJSON(w, 400, map[string]any{"error": "invalid type"})
		return
	}
	if b.Severity != "" && !journal.ValidSeverity(b.Severity) {
		writeJSON(w, 400, map[string]any{"error": "invalid severity"})
		return
	}
	if len(b.Source) > 128 || len(b.Key) > 128 || strings.ContainsAny(b.Source+b.Key, "\x00\n\r\t") {
		writeJSON(w, 400, map[string]any{"error": "invalid source or idempotency_key"})
		return
	}
	orig, ok := s.pipe.Journal.Get(b.Seq)
	if !ok {
		writeJSON(w, 404, map[string]any{"error": "seq not retained in the journal", "seq": b.Seq})
		return
	}
	data, err := journal.ApplyPaths(orig.Data, b.Set, b.Remove)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	evType, sev, src := orig.Type, orig.Severity, orig.Source
	if b.Type != "" {
		evType = b.Type
	}
	if b.Severity != "" {
		sev = b.Severity
	}
	if b.Source != "" {
		src = b.Source
	}
	type result struct {
		Seq     int64  `json:"seq"`
		Key     string `json:"idempotency_key"`
		Outcome string `json:"outcome"`
		Error   string `json:"error,omitempty"`
	}
	results := make([]result, 0, b.Count)
	failed := 0
	for n := 1; n <= b.Count; n++ {
		key := b.Key
		switch {
		case key == "":
			key = s.simKey(fmt.Sprintf("resend/%d", b.Seq))
		case b.Count > 1:
			key = fmt.Sprintf("%s-%d", key, n)
		}
		s.IncPublish()
		seq, err := s.emitSeq("resend", evType, sev, src, key, data)
		res := result{Seq: seq, Key: key}
		if err != nil {
			res.Error = err.Error()
			failed++
		}
		if e, ok := s.pipe.Journal.Get(seq); ok {
			res.Outcome = e.Outcome
		}
		results = append(results, res)
	}
	code := 200
	if failed == b.Count {
		code = 502
	}
	writeJSON(w, code, map[string]any{"source_seq": b.Seq, "type": evType, "severity": sev, "data": data, "results": results, "failed": failed})
}
