// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
)

type Server struct {
	Store *season.Store
	Pub   *relaypub.Client
	Mux   *http.ServeMux
}

func New(store *season.Store, pub *relaypub.Client) *Server {
	s := &Server{Store: store, Pub: pub, Mux: http.NewServeMux()}
	s.Mux.HandleFunc("GET /healthz", s.health)
	s.Mux.HandleFunc("GET /readyz", s.health)
	s.Mux.HandleFunc("GET /v1/seasons", s.listSeasons)
	s.Mux.HandleFunc("POST /v1/seasons", s.createSeason)
	s.Mux.HandleFunc("GET /v1/seasons/{id}", s.getSeason)
	s.Mux.HandleFunc("PUT /v1/seasons/{id}", s.putSeason)
	s.Mux.HandleFunc("DELETE /v1/seasons/{id}", s.deleteSeason)
	s.Mux.HandleFunc("POST /v1/seasons/{id}/open", s.openSeason)
	s.Mux.HandleFunc("POST /v1/seasons/{id}/close", s.closeSeason)
	s.Mux.HandleFunc("POST /v1/seasons/{id}/events", s.publishSeasonEvent)
	return s
}

func (s *Server) Handler() http.Handler { return s.Mux }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "relay-edge",
		"time":    time.Now().UTC(),
	})
}

func (s *Server) listSeasons(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.Store.List()})
}

func (s *Server) getSeason(w http.ResponseWriter, r *http.Request) {
	it, err := s.Store.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, 200, it)
}

func (s *Server) createSeason(w http.ResponseWriter, r *http.Request) {
	var in season.Season
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if in.ID == "" {
		in.ID = "season_" + fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)
	}
	out, err := s.Store.Put(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, out)
}

func (s *Server) putSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in season.Season
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	in.ID = id
	out, err := s.Store.Put(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) deleteSeason(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Delete(r.PathValue("id")); err != nil {
		if errors.Is(err, season.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) openSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, err := s.Store.Get(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	if it.Status == "active" {
		writeJSON(w, 200, map[string]any{"season": it, "note": "already active"})
		return
	}
	key := fmt.Sprintf("edge/season/%s/open/%d", id, time.Now().Unix())
	data := seasonEventData(it, map[string]any{
		"advisory": fmt.Sprintf("Season %s opened for crop %s at site %s", it.Name, it.Crop, it.Site),
	})
	res, err := s.Pub.PublishEventType("crop.advisory", "info", seasonSource(it), key, data)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	updated, _ := s.Store.UpdateStatus(id, "active", res.EventID)
	writeJSON(w, 200, map[string]any{"season": updated, "publish": res})
}

func (s *Server) closeSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, err := s.Store.Get(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	key := fmt.Sprintf("edge/season/%s/close/%d", id, time.Now().Unix())
	data := seasonEventData(it, map[string]any{
		"advisory": fmt.Sprintf("Season %s closed", it.Name),
	})
	res, err := s.Pub.PublishEventType("crop.advisory", "info", seasonSource(it), key, data)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	updated, _ := s.Store.UpdateStatus(id, "closed", res.EventID)
	writeJSON(w, 200, map[string]any{"season": updated, "publish": res})
}

type seasonEventIn struct {
	Type           string         `json:"type"`
	Severity       string         `json:"severity"`
	IdempotencyKey string         `json:"idempotency_key"`
	Data           map[string]any `json:"data"`
	Command        string         `json:"command,omitempty"` // optional critical action command
	Zone           string         `json:"zone,omitempty"`
}

func (s *Server) publishSeasonEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, err := s.Store.Get(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	if it.Status != "active" {
		writeJSON(w, 409, map[string]any{"error": "season must be active to publish critical farm events", "status": it.Status})
		return
	}
	var in seasonEventIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if in.Type == "" {
		writeJSON(w, 400, map[string]any{"error": "type required"})
		return
	}
	if in.Severity == "" {
		in.Severity = "info"
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = fmt.Sprintf("edge/season/%s/%s/%d", id, in.Type, time.Now().UnixNano())
	}
	extra := in.Data
	if extra == nil {
		extra = map[string]any{}
	}
	if in.Command != "" {
		zone := in.Zone
		if zone == "" {
			zone = "A4"
		}
		extra["recommended_action"] = map[string]any{
			"target":  "farm-controller",
			"command": in.Command,
			"payload": map[string]any{"zone": zone, "season_id": it.ID},
		}
		if in.Severity == "info" {
			in.Severity = "critical"
		}
	}
	data := seasonEventData(it, extra)
	res, err := s.Pub.PublishEventType(in.Type, in.Severity, seasonSource(it), in.IdempotencyKey, data)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if res.EventID != "" {
		_, _ = s.Store.UpdateStatus(it.ID, it.Status, res.EventID)
	}
	writeJSON(w, 200, map[string]any{"season_id": it.ID, "publish": res})
}

func seasonSource(it season.Season) string {
	parts := []string{}
	if it.Site != "" {
		parts = append(parts, it.Site)
	}
	parts = append(parts, it.Name)
	if it.Crop != "" {
		parts = append(parts, it.Crop)
	}
	return strings.Join(parts, " / ")
}

func seasonEventData(it season.Season, extra map[string]any) map[string]any {
	out := map[string]any{
		"season_id":   it.ID,
		"season_name": it.Name,
		"crop":        it.Crop,
		"site":        it.Site,
		"recipient":   "demo-device-token",
	}
	for k, v := range it.Labels {
		out["label_"+k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
