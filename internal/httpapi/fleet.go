// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zyvorai/relay-edge/internal/fleet"
)

func (s *Server) mountFleet() {
	s.fleetPublish = false
	s.fleetInterval = 2000
	s.fleetSubs = map[chan []byte]struct{}{}
	s.Fleet = fleet.New(s.onFleetTick)

	s.Mux.HandleFunc("GET /v1/fleet/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"classes": fleet.Classes(), "items": fleet.Catalog()})
	})
	s.Mux.HandleFunc("GET /v1/fleet/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, s.fleetSnapshot())
	})
	s.Mux.HandleFunc("GET /v1/fleet/events", s.fleetEvents)
	s.Mux.HandleFunc("GET /v1/fleet/stream", s.fleetStream)
	s.Mux.HandleFunc("GET /v1/fleet/config", s.fleetGetConfig)
	s.Mux.HandleFunc("POST /v1/fleet/config", s.fleetConfig)
	s.Mux.HandleFunc("POST /v1/fleet/tick", func(w http.ResponseWriter, _ *http.Request) {
		snap := s.Fleet.Tick()
		evts := s.processFleetTick(snap)
		writeJSON(w, 200, map[string]any{"snapshot": s.fleetSnapshotFrom(snap), "events": evts})
	})
	s.Mux.HandleFunc("POST /v1/fleet/start", func(w http.ResponseWriter, _ *http.Request) {
		s.Fleet.Start()
		writeJSON(w, 200, map[string]any{"running": true, "snapshot": s.fleetSnapshot()})
	})
	s.Mux.HandleFunc("POST /v1/fleet/stop", func(w http.ResponseWriter, _ *http.Request) {
		s.Fleet.Stop()
		writeJSON(w, 200, map[string]any{"running": false, "snapshot": s.fleetSnapshot()})
	})
	s.Mux.HandleFunc("POST /v1/fleet/scenario", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scenario string `json:"scenario"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Scenario == "" {
			body.Scenario = "nominal"
		}
		snap := s.Fleet.SetScenario(body.Scenario)
		evts := s.processFleetTick(snap)
		writeJSON(w, 200, map[string]any{"snapshot": s.fleetSnapshotFrom(snap), "events": evts})
	})
}

func (s *Server) fleetSnapshot() map[string]any {
	return s.fleetSnapshotFrom(s.Fleet.Snapshot())
}

func (s *Server) fleetSnapshotFrom(snap fleet.Snapshot) map[string]any {
	return map[string]any{
		"scenario":    snap.Scenario,
		"running":     snap.Running,
		"classes":     snap.Classes,
		"readings":    snap.Readings,
		"values":      snap.Values,
		"updated_at":  snap.UpdatedAt,
		"publish":     s.fleetPublish,
		"interval_ms": s.fleetInterval,
	}
}

func (s *Server) fleetGetConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"publish": s.fleetPublish, "interval_ms": s.fleetInterval})
}

func (s *Server) fleetConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Publish    bool `json:"publish"`
		IntervalMS int  `json:"interval_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.fleetPublish = body.Publish
	if body.IntervalMS >= 250 {
		s.fleetInterval = body.IntervalMS
		s.Fleet.SetInterval(body.IntervalMS)
	}
	writeJSON(w, 200, map[string]any{"publish": s.fleetPublish, "interval_ms": s.fleetInterval})
}

func (s *Server) fleetEvents(w http.ResponseWriter, _ *http.Request) {
	s.fleetMu.Lock()
	defer s.fleetMu.Unlock()
	writeJSON(w, 200, map[string]any{"items": s.fleetEventsLog})
}

func (s *Server) fleetStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]any{"error": "stream unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 16)
	s.fleetMu.Lock()
	s.fleetSubs[ch] = struct{}{}
	s.fleetMu.Unlock()
	defer func() {
		s.fleetMu.Lock()
		delete(s.fleetSubs, ch)
		s.fleetMu.Unlock()
	}()
	init, _ := json.Marshal(map[string]any{"kind": "snapshot", "snapshot": s.fleetSnapshot()})
	fmt.Fprintf(w, "data: %s\n\n", init)
	fl.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}
	}
}

func (s *Server) onFleetTick(snap fleet.Snapshot) {
	s.processFleetTick(snap)
}

func (s *Server) processFleetTick(snap fleet.Snapshot) []fleet.Event {
	evts := fleet.Derive(snap)
	stored := make([]fleet.Event, 0, len(evts))
	for _, ev := range evts {
		s.appendFleetEvent(ev)
		stored = append(stored, ev)
		if s.fleetPublish {
			s.publishSimEvent(ev.Type, ev.Severity, ev.Command, ev.DeviceID, "fleet", ev.Data)
		}
	}
	s.broadcastFleet(map[string]any{"kind": "tick", "snapshot": s.fleetSnapshotFrom(snap), "events": stored})
	return stored
}

func (s *Server) appendFleetEvent(ev fleet.Event) {
	s.fleetMu.Lock()
	defer s.fleetMu.Unlock()
	s.fleetEventsLog = append([]fleet.Event{ev}, s.fleetEventsLog...)
	if len(s.fleetEventsLog) > 400 {
		s.fleetEventsLog = s.fleetEventsLog[:400]
	}
}

func (s *Server) broadcastFleet(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.fleetMu.Lock()
	defer s.fleetMu.Unlock()
	for ch := range s.fleetSubs {
		select {
		case ch <- b:
		default:
		}
	}
}
