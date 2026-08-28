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
	s.Mux.HandleFunc("GET /v1/atlas/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"items": atlas.Catalog()})
	})
	s.Mux.HandleFunc("GET /v1/atlas/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, s.Atlas.Snapshot())
	})
	s.Mux.HandleFunc("POST /v1/atlas/tick", func(w http.ResponseWriter, _ *http.Request) {
		snap := s.Atlas.Tick()
		writeJSON(w, 200, map[string]any{"snapshot": snap, "events": atlas.Derive(snap)})
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
		writeJSON(w, 200, map[string]any{"snapshot": snap, "events": atlas.Derive(snap)})
	})
}
