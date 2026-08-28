// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/zyvorai/relay-edge/internal/atlas"
)

func (s *Server) mountAtlas() {
	s.Atlas = atlas.New()
	s.atlasPublish = false
	s.Mux.HandleFunc("GET /v1/atlas/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"items": atlas.Catalog()})
	})
	s.Mux.HandleFunc("GET /v1/atlas/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, s.Atlas.Snapshot())
	})
	s.Mux.HandleFunc("POST /v1/atlas/config", s.atlasConfig)
	s.Mux.HandleFunc("POST /v1/atlas/tick", func(w http.ResponseWriter, _ *http.Request) {
		snap := s.Atlas.Tick()
		evts := atlas.Derive(snap)
		s.publishAtlasEvents(evts)
		writeJSON(w, 200, map[string]any{"snapshot": snap, "events": evts})
	})
	s.Mux.HandleFunc("POST /v1/atlas/start", func(w http.ResponseWriter, _ *http.Request) {
		s.Atlas.Start()
		writeJSON(w, 200, s.Atlas.Snapshot())
	})
	s.Mux.HandleFunc("POST /v1/atlas/stop", func(w http.ResponseWriter, _ *http.Request) {
		s.Atlas.Stop()
		writeJSON(w, 200, s.Atlas.Snapshot())
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
		evts := atlas.Derive(snap)
		s.publishAtlasEvents(evts)
		writeJSON(w, 200, map[string]any{"snapshot": snap, "events": evts})
	})
}

func (s *Server) atlasConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Publish bool `json:"publish"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.atlasPublish = body.Publish
	writeJSON(w, 200, map[string]any{"publish": s.atlasPublish})
}

func (s *Server) publishAtlasEvents(evts []atlas.Event) {
	if !s.atlasPublish {
		return
	}
	for _, ev := range evts {
		s.publishSimEvent(ev.Type, ev.Severity, ev.Command, ev.DeviceID, "atlas", ev.Data)
	}
}
