// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zyvorai/relay-edge/internal/atlas"
)

func (s *Server) mountAtlas() {
	s.atlasPublish = false
	s.atlasInterval = 2000
	s.atlasSubs = map[chan []byte]struct{}{}
	s.Atlas = atlas.New(s.onAtlasTick)

	s.Mux.HandleFunc("GET /v1/atlas/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"items": atlas.Catalog()})
	})
	s.Mux.HandleFunc("GET /v1/atlas/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, s.atlasSnapshot())
	})
	s.Mux.HandleFunc("GET /v1/atlas/events", s.atlasEvents)
	s.Mux.HandleFunc("GET /v1/atlas/stream", s.atlasStream)
	s.Mux.HandleFunc("GET /v1/atlas/config", s.atlasGetConfig)
	s.Mux.HandleFunc("POST /v1/atlas/config", s.atlasConfig)
	s.Mux.HandleFunc("POST /v1/atlas/tick", func(w http.ResponseWriter, _ *http.Request) {
		snap := s.Atlas.Tick()
		evts := s.processAtlasTick(snap)
		writeJSON(w, 200, map[string]any{"snapshot": s.atlasSnapshotFrom(snap), "events": evts})
	})
	s.Mux.HandleFunc("POST /v1/atlas/start", func(w http.ResponseWriter, _ *http.Request) {
		s.Atlas.Start()
		writeJSON(w, 200, map[string]any{"running": true, "snapshot": s.atlasSnapshot()})
	})
	s.Mux.HandleFunc("POST /v1/atlas/stop", func(w http.ResponseWriter, _ *http.Request) {
		s.Atlas.Stop()
		writeJSON(w, 200, map[string]any{"running": false, "snapshot": s.atlasSnapshot()})
	})
	s.Mux.HandleFunc("POST /v1/atlas/scenario", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scenario string `json:"scenario"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Scenario == "" {
			body.Scenario = "nominal"
		}
		snap := s.Atlas.SetScenario(body.Scenario)
		evts := s.processAtlasTick(snap)
		writeJSON(w, 200, map[string]any{"snapshot": s.atlasSnapshotFrom(snap), "events": evts})
	})
}

func (s *Server) atlasSnapshot() map[string]any {
	return s.atlasSnapshotFrom(s.Atlas.Snapshot())
}

func (s *Server) atlasSnapshotFrom(snap atlas.Snapshot) map[string]any {
	return map[string]any{
		"scenario":    snap.Scenario,
		"running":     snap.Running,
		"readings":    snap.Readings,
		"values":      snap.Values,
		"link_mode":   snap.LinkMode,
		"updated_at":  snap.UpdatedAt,
		"publish":     s.atlasPublish,
		"interval_ms": s.atlasInterval,
	}
}

func (s *Server) atlasGetConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"publish": s.atlasPublish, "interval_ms": s.atlasInterval})
}

func (s *Server) atlasConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Publish    bool `json:"publish"`
		IntervalMS int  `json:"interval_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.atlasPublish = body.Publish
	if body.IntervalMS >= 250 {
		s.atlasInterval = body.IntervalMS
		s.Atlas.SetInterval(body.IntervalMS)
	}
	writeJSON(w, 200, map[string]any{"publish": s.atlasPublish, "interval_ms": s.atlasInterval})
}

func (s *Server) atlasEvents(w http.ResponseWriter, _ *http.Request) {
	s.atlasMu.Lock()
	defer s.atlasMu.Unlock()
	writeJSON(w, 200, map[string]any{"items": s.atlasEventsLog})
}

func (s *Server) atlasStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]any{"error": "stream unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 16)
	s.atlasMu.Lock()
	s.atlasSubs[ch] = struct{}{}
	s.atlasMu.Unlock()
	defer func() {
		s.atlasMu.Lock()
		delete(s.atlasSubs, ch)
		s.atlasMu.Unlock()
	}()
	init, _ := json.Marshal(map[string]any{"kind": "snapshot", "snapshot": s.atlasSnapshot()})
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

func (s *Server) onAtlasTick(snap atlas.Snapshot) {
	s.processAtlasTick(snap)
}

func (s *Server) processAtlasTick(snap atlas.Snapshot) []atlas.Event {
	evts := atlas.Derive(snap)
	stored := make([]atlas.Event, 0, len(evts))
	for _, ev := range evts {
		s.appendAtlasEvent(ev)
		stored = append(stored, ev)
		if s.atlasPublish {
			s.publishSimEvent(ev.Type, ev.Severity, ev.Command, ev.DeviceID, "atlas", ev.Data)
		}
	}
	s.broadcastAtlas(map[string]any{"kind": "tick", "snapshot": s.atlasSnapshotFrom(snap), "events": stored})
	return stored
}

func (s *Server) appendAtlasEvent(ev atlas.Event) {
	s.atlasMu.Lock()
	defer s.atlasMu.Unlock()
	s.atlasEventsLog = append([]atlas.Event{ev}, s.atlasEventsLog...)
	if len(s.atlasEventsLog) > 400 {
		s.atlasEventsLog = s.atlasEventsLog[:400]
	}
}

func (s *Server) broadcastAtlas(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.atlasMu.Lock()
	defer s.atlasMu.Unlock()
	for ch := range s.atlasSubs {
		select {
		case ch <- b:
		default:
		}
	}
}
