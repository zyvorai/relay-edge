// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/zyvorai/relay-edge/internal/fleet"
)

func (s *Server) mountFleet() {
	s.Fleet = fleet.New()
	s.fleetPublish = false
	s.Mux.HandleFunc("GET /v1/fleet/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"classes": fleet.Classes(), "items": fleet.Catalog()})
	})
	s.Mux.HandleFunc("GET /v1/fleet/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, s.Fleet.Snapshot())
	})
	s.Mux.HandleFunc("POST /v1/fleet/config", s.fleetConfig)
	s.Mux.HandleFunc("POST /v1/fleet/tick", func(w http.ResponseWriter, _ *http.Request) {
		snap := s.Fleet.Tick()
		evts := fleet.Derive(snap)
		s.publishFleetEvents(evts)
		writeJSON(w, 200, map[string]any{"snapshot": snap, "events": evts})
	})
	s.Mux.HandleFunc("POST /v1/fleet/start", func(w http.ResponseWriter, _ *http.Request) {
		s.Fleet.Start()
		writeJSON(w, 200, s.Fleet.Snapshot())
	})
	s.Mux.HandleFunc("POST /v1/fleet/stop", func(w http.ResponseWriter, _ *http.Request) {
		s.Fleet.Stop()
		writeJSON(w, 200, s.Fleet.Snapshot())
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
		evts := fleet.Derive(snap)
		s.publishFleetEvents(evts)
		writeJSON(w, 200, map[string]any{"snapshot": snap, "events": evts})
	})
}

func (s *Server) fleetConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Publish bool `json:"publish"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.fleetPublish = body.Publish
	writeJSON(w, 200, map[string]any{"publish": s.fleetPublish})
}

func (s *Server) publishFleetEvents(evts []fleet.Event) {
	if !s.fleetPublish {
		return
	}
	for _, ev := range evts {
		s.publishSimEvent(ev.Type, ev.Severity, ev.Command, ev.DeviceID, "fleet", ev.Data)
	}
}
