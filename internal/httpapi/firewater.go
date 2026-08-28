// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/zyvorai/relay-edge/internal/firewater"
	"github.com/zyvorai/relay-edge/web"
)

func (s *Server) mountFirewater() {
	s.FW = firewater.New(s.onFirewaterTick)
	s.fwSubs = map[chan []byte]struct{}{}

	s.Mux.HandleFunc("GET /v1/firewater/snapshot", s.fwSnapshot)
	s.Mux.HandleFunc("GET /v1/firewater/events", s.fwEvents)
	s.Mux.HandleFunc("GET /v1/firewater/catalog", s.fwCatalog)
	s.Mux.HandleFunc("GET /v1/firewater/stream", s.fwStream)
	s.Mux.HandleFunc("POST /v1/firewater/seed", s.fwSeed)
	s.Mux.HandleFunc("POST /v1/firewater/start", s.fwStart)
	s.Mux.HandleFunc("POST /v1/firewater/stop", s.fwStop)
	s.Mux.HandleFunc("POST /v1/firewater/tick", s.fwTick)
	s.Mux.HandleFunc("POST /v1/firewater/scenario", s.fwScenario)
	s.Mux.HandleFunc("POST /v1/firewater/config", s.fwConfig)
	s.Mux.HandleFunc("GET /v1/firewater/topology", s.fwTopology)
	s.Mux.HandleFunc("GET /v1/firewater/matrix", s.fwMatrix)
	s.Mux.HandleFunc("GET /v1/firewater/ready", s.fwReady)
	s.Mux.HandleFunc("GET /v1/firewater/alarms", s.fwAlarms)
	s.Mux.HandleFunc("POST /v1/firewater/alarms/{id}/ack", s.fwAck)
	s.Mux.HandleFunc("POST /v1/firewater/alarms/{id}/shelve", s.fwShelve)
	s.Mux.HandleFunc("POST /v1/firewater/act", s.fwAct)
	s.Mux.HandleFunc("GET /v1/firewater/verify", s.fwVerify)
	s.Mux.HandleFunc("GET /v1/firewater/sparkplug", s.fwSparkplug)
	s.Mux.HandleFunc("GET /v1/firewater/modbus", s.fwModbus)
	s.Mux.HandleFunc("POST /v1/firewater/weekly-test", s.fwWeekly)

	ui, err := fs.Sub(web.FS, ".")
	if err != nil {
		log.Printf("ui embed: %v", err)
		return
	}
	s.Mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(ui))))
	s.Mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
	})
}

func (s *Server) fwSnapshot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.FW.Snapshot())
}

func (s *Server) fwCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"items": firewater.Catalog()})
}

func (s *Server) fwEvents(w http.ResponseWriter, _ *http.Request) {
	s.fwMu.Lock()
	defer s.fwMu.Unlock()
	writeJSON(w, 200, map[string]any{"items": s.fwEventsLog})
}

func (s *Server) fwSeed(w http.ResponseWriter, _ *http.Request) {
	out, err := firewater.Seed(s.Sites, s.Devices, s.Contacts, s.Seasons)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) fwStart(w http.ResponseWriter, _ *http.Request) {
	s.FW.Start()
	writeJSON(w, 200, map[string]any{"running": true, "snapshot": s.FW.Snapshot()})
}

func (s *Server) fwStop(w http.ResponseWriter, _ *http.Request) {
	s.FW.Stop()
	writeJSON(w, 200, map[string]any{"running": false})
}

func (s *Server) fwTick(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.FW.Tick())
}

func (s *Server) fwScenario(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scenario string `json:"scenario"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Scenario == "" {
		writeJSON(w, 400, map[string]any{"error": "scenario required"})
		return
	}
	s.FW.SetScenario(body.Scenario)
	writeJSON(w, 200, s.FW.Tick())
}

func (s *Server) fwTopology(w http.ResponseWriter, _ *http.Request) {
	g := firewater.PlantGraph()
	writeJSON(w, 200, map[string]any{"graph": g, "if_valve_shut": g.Downstream("valve_a")})
}

func (s *Server) fwMatrix(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"rules": firewater.Matrix()})
}

func (s *Server) fwReady(w http.ResponseWriter, _ *http.Request) {
	v := s.FW.Values()
	writeJSON(w, 200, map[string]any{"system_ready": firewater.SystemReady(v), "why": firewater.ReadyWhy(v)})
}

func (s *Server) fwAlarms(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.FW.Book().List()})
}

func (s *Server) fwAck(w http.ResponseWriter, r *http.Request) {
	a := s.FW.Book().Ack(r.PathValue("id"))
	if a == nil {
		writeJSON(w, 404, map[string]any{"error": "alarm not found"})
		return
	}
	writeJSON(w, 200, a)
}

func (s *Server) fwShelve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Minutes int `json:"minutes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Minutes <= 0 {
		body.Minutes = 15
	}
	a := s.FW.Book().Shelve(r.PathValue("id"), time.Duration(body.Minutes)*time.Minute)
	if a == nil {
		writeJSON(w, 404, map[string]any{"error": "alarm not found"})
		return
	}
	writeJSON(w, 200, a)
}

func (s *Server) fwAct(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Command == "" {
		writeJSON(w, 400, map[string]any{"error": "command required"})
		return
	}
	d := firewater.Evaluate(body.Command, s.FW.Values())
	code := 200
	if !d.Allowed {
		code = 409
	}
	writeJSON(w, code, d)
}

func (s *Server) fwVerify(w http.ResponseWriter, r *http.Request) {
	cmd := r.URL.Query().Get("command")
	if cmd == "" {
		writeJSON(w, 400, map[string]any{"error": "command query required"})
		return
	}
	writeJSON(w, 200, firewater.Verify(cmd, s.FW.Values()))
}

func (s *Server) fwSparkplug(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, firewater.Sparkplug("fasal-onprem", "fw-edge-01", s.FW.NextSeq(), s.FW.Values()))
}

func (s *Server) fwModbus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"holding": firewater.ModbusMap(s.FW.Values()), "unit_id": 1})
}

func (s *Server) fwWeekly(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, firewater.RunWeeklyTest(s.FW.Values()))
}

func (s *Server) fwConfig(w http.ResponseWriter, r *http.Request) {
	var cfg firewater.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.FW.Apply(cfg)
	writeJSON(w, 200, s.FW.Config())
}

func (s *Server) fwStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]any{"error": "stream unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 16)
	s.fwMu.Lock()
	s.fwSubs[ch] = struct{}{}
	s.fwMu.Unlock()
	defer func() {
		s.fwMu.Lock()
		delete(s.fwSubs, ch)
		s.fwMu.Unlock()
	}()
	init, _ := json.Marshal(map[string]any{"kind": "snapshot", "snapshot": s.FW.Snapshot()})
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

func (s *Server) onFirewaterTick(evts []firewater.Event, snap firewater.Snapshot) {
	stored := make([]firewater.Event, 0, len(evts))
	for _, ev := range evts {
		s.appendFWEvent(ev)
		stored = append(stored, ev)
		if s.FW.PublishEnabled() {
			s.publishFW(ev)
		}
	}
	s.broadcastFW(map[string]any{"kind": "tick", "snapshot": snap, "events": stored})
}

func (s *Server) appendFWEvent(ev firewater.Event) {
	s.fwMu.Lock()
	defer s.fwMu.Unlock()
	s.fwEventsLog = append([]firewater.Event{ev}, s.fwEventsLog...)
	if len(s.fwEventsLog) > 400 {
		s.fwEventsLog = s.fwEventsLog[:400]
	}
}

func (s *Server) broadcastFW(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.fwMu.Lock()
	defer s.fwMu.Unlock()
	for ch := range s.fwSubs {
		select {
		case ch <- b:
		default:
		}
	}
}

func (s *Server) publishFW(ev firewater.Event) {
	it, err := s.Seasons.Get(firewater.SeasonID)
	if err != nil {
		log.Printf("firewater publish: season %s missing (POST /v1/firewater/seed first)", firewater.SeasonID)
		return
	}
	ctx := s.resolveEnrich(it, ev.ZoneID, firewater.ZoneCode, ev.DeviceID)
	key := fmt.Sprintf("edge/firewater/%s/%s/%d", ev.Type, ev.DeviceID, time.Now().UnixNano())
	extra := ev.Data
	if extra == nil {
		extra = map[string]any{}
	}
	if ev.Command != "" {
		extra["recommended_action"] = map[string]any{
			"target":  "firewater-controller",
			"command": ev.Command,
			"payload": map[string]any{
				"zone":      firewater.ZoneCode,
				"zone_id":   ev.ZoneID,
				"device_id": ev.DeviceID,
				"season_id": it.ID,
			},
		}
	}
	data := s.stampData(ctx, extra)
	if _, err := s.Pub.PublishEventType(ev.Type, string(ev.Severity), seasonSource(ctx), key, data); err != nil {
		log.Printf("firewater publish %s: %v", ev.Type, err)
	}
}
