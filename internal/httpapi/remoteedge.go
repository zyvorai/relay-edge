// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zyvorai/relay-edge/internal/remoteedge"
)

func (s *Server) mountRemoteEdge() {
	s.remoteEdgePublish = false
	s.remoteEdgeInterval = 2000
	s.remoteEdgeSubs = map[chan []byte]struct{}{}
	s.RemoteEdge = remoteedge.New(s.onRemoteEdgeTick)

	s.Mux.HandleFunc("GET /v1/remote-edge/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"items": remoteedge.Catalog()})
	})
	s.Mux.HandleFunc("GET /v1/remote-edge/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, s.remoteEdgeSnapshot())
	})
	s.Mux.HandleFunc("GET /v1/remote-edge/events", s.remoteEdgeEvents)
	s.Mux.HandleFunc("GET /v1/remote-edge/stream", s.remoteEdgeStream)
	s.Mux.HandleFunc("GET /v1/remote-edge/config", s.remoteEdgeGetConfig)
	s.Mux.HandleFunc("POST /v1/remote-edge/config", s.remoteEdgeConfig)
	s.Mux.HandleFunc("POST /v1/remote-edge/tick", func(w http.ResponseWriter, _ *http.Request) {
		snap := s.RemoteEdge.Tick()
		evts := s.processRemoteEdgeTick(snap)
		writeJSON(w, 200, map[string]any{"snapshot": s.remoteEdgeSnapshotFrom(snap), "events": evts})
	})
	s.Mux.HandleFunc("POST /v1/remote-edge/start", func(w http.ResponseWriter, _ *http.Request) {
		s.RemoteEdge.Start()
		snap := s.remoteEdgeSnapshot()
		s.broadcastRemoteEdge(map[string]any{"kind": "start", "snapshot": snap, "events": []any{}})
		writeJSON(w, 200, map[string]any{"running": true, "snapshot": snap})
	})
	s.Mux.HandleFunc("POST /v1/remote-edge/stop", func(w http.ResponseWriter, _ *http.Request) {
		s.RemoteEdge.Stop()
		snap := s.remoteEdgeSnapshot()
		s.broadcastRemoteEdge(map[string]any{"kind": "stop", "snapshot": snap, "events": []any{}})
		writeJSON(w, 200, map[string]any{"running": false, "snapshot": snap})
	})
	s.Mux.HandleFunc("POST /v1/remote-edge/scenario", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scenario string `json:"scenario"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Scenario == "" {
			body.Scenario = "nominal"
		}
		snap := s.RemoteEdge.SetScenario(body.Scenario)
		evts := s.processRemoteEdgeTick(snap)
		writeJSON(w, 200, map[string]any{"snapshot": s.remoteEdgeSnapshotFrom(snap), "events": evts})
	})
}

func (s *Server) remoteEdgeSnapshot() map[string]any {
	return s.remoteEdgeSnapshotFrom(s.RemoteEdge.Snapshot())
}

func (s *Server) remoteEdgeSnapshotFrom(snap remoteedge.Snapshot) map[string]any {
	return map[string]any{
		"scenario":    snap.Scenario,
		"running":     snap.Running,
		"readings":    snap.Readings,
		"values":      snap.Values,
		"link_mode":   snap.LinkMode,
		"updated_at":  snap.UpdatedAt,
		"publish":     s.remoteEdgePublish,
		"interval_ms": s.remoteEdgeInterval,
	}
}

func (s *Server) remoteEdgeGetConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"publish": s.remoteEdgePublish, "interval_ms": s.remoteEdgeInterval})
}

func (s *Server) remoteEdgeConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Publish    bool `json:"publish"`
		IntervalMS int  `json:"interval_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.remoteEdgePublish = body.Publish
	if body.IntervalMS >= 250 {
		s.remoteEdgeInterval = body.IntervalMS
		s.RemoteEdge.SetInterval(body.IntervalMS)
	}
	writeJSON(w, 200, map[string]any{"publish": s.remoteEdgePublish, "interval_ms": s.remoteEdgeInterval})
}

func (s *Server) remoteEdgeEvents(w http.ResponseWriter, _ *http.Request) {
	s.remoteEdgeMu.Lock()
	defer s.remoteEdgeMu.Unlock()
	writeJSON(w, 200, map[string]any{"items": s.remoteEdgeEventsLog})
}

func (s *Server) remoteEdgeStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]any{"error": "stream unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 16)
	s.remoteEdgeMu.Lock()
	s.remoteEdgeSubs[ch] = struct{}{}
	s.remoteEdgeMu.Unlock()
	defer func() {
		s.remoteEdgeMu.Lock()
		delete(s.remoteEdgeSubs, ch)
		s.remoteEdgeMu.Unlock()
	}()
	init, _ := json.Marshal(map[string]any{"kind": "snapshot", "snapshot": s.remoteEdgeSnapshot()})
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

func (s *Server) onRemoteEdgeTick(snap remoteedge.Snapshot) {
	s.processRemoteEdgeTick(snap)
}

func (s *Server) processRemoteEdgeTick(snap remoteedge.Snapshot) []remoteedge.Event {
	evts := remoteedge.Derive(snap)
	stored := make([]remoteedge.Event, 0, len(evts))
	for _, ev := range evts {
		s.appendRemoteEdgeEvent(ev)
		stored = append(stored, ev)
		if s.remoteEdgePublish {
			s.publishSimEvent(ev.Type, ev.Severity, ev.Command, ev.DeviceID, "remote-edge", ev.Data)
		}
	}
	s.broadcastRemoteEdge(map[string]any{"kind": "tick", "snapshot": s.remoteEdgeSnapshotFrom(snap), "events": stored})
	return stored
}

func (s *Server) appendRemoteEdgeEvent(ev remoteedge.Event) {
	s.remoteEdgeMu.Lock()
	defer s.remoteEdgeMu.Unlock()
	s.remoteEdgeEventsLog = append([]remoteedge.Event{ev}, s.remoteEdgeEventsLog...)
	if len(s.remoteEdgeEventsLog) > 400 {
		s.remoteEdgeEventsLog = s.remoteEdgeEventsLog[:400]
	}
}

func (s *Server) broadcastRemoteEdge(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.remoteEdgeMu.Lock()
	defer s.remoteEdgeMu.Unlock()
	for ch := range s.remoteEdgeSubs {
		select {
		case ch <- b:
		default:
		}
	}
}
