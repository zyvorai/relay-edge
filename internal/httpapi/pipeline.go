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
	p := s.pipe
	e := p.Journal.Add(journal.Entry{Type: evType, Severity: severity, Source: source, Key: key, Data: data, Origin: origin})
	d := p.Chaos.Decide()
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
		return nil
	case chaos.Error:
		p.Journal.SetOutcome(e.Seq, "error")
		return chaos.ErrInjected
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
		return nil
	} else if d.Delay > 0 {
		p.pending.Add(-1)
	}
	return run()
}

func (s *Server) mountPipeline() {
	s.Mux.HandleFunc("GET /v1/sim/seed", s.getSimSeed)
	s.Mux.HandleFunc("POST /v1/sim/seed", s.postSimSeed)
	s.Mux.HandleFunc("GET /v1/journal", s.listJournal)
	s.Mux.HandleFunc("GET /v1/journal/export", s.exportJournal)
	s.Mux.HandleFunc("POST /v1/journal/clear", s.clearJournal)
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
	if len(events) == 0 {
		writeJSON(w, 400, map[string]any{"error": "no events selected"})
		return
	}
	opts := replay.Options{Speed: b.Speed, MaxGap: time.Duration(b.MaxGapMS) * time.Millisecond, RunID: b.RunID, DryRun: b.DryRun}
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
