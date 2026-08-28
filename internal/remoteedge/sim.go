// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package remoteedge

import (
	"math/rand"
	"sync"
	"time"
)

type Event struct {
	Type     string         `json:"type"`
	Severity string         `json:"severity"`
	Command  string         `json:"command,omitempty"`
	DeviceID string         `json:"device_id"`
	Data     map[string]any `json:"data"`
	At       time.Time      `json:"created_at"`
}

type Reading struct {
	ID, Name, Class, Unit, Protocol, Vendor string
	Value                                   float64
	Severity                                string
}

type Snapshot struct {
	Scenario  string             `json:"scenario"`
	Running   bool               `json:"running"`
	Readings  []Reading          `json:"readings"`
	Values    map[string]float64 `json:"values"`
	LinkMode  string             `json:"link_mode"` // starlink|lte|p5g|offline
	UpdatedAt time.Time          `json:"updated_at"`
}

type TickHandler func(Snapshot)

type Engine struct {
	mu       sync.Mutex
	scenario string
	running  bool
	values   map[string]float64
	stop     chan struct{}
	interval time.Duration
	onStep   TickHandler
}

func New(onStep TickHandler) *Engine {
	return &Engine{scenario: "nominal", values: nominal(), interval: 2 * time.Second, onStep: onStep}
}

func (e *Engine) SetInterval(ms int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ms >= 250 {
		e.interval = time.Duration(ms) * time.Millisecond
	}
}

func nominal() map[string]float64 {
	return map[string]float64{
		"beacon_ok": 1, "beacon_cpu": 28, "cruiser_gpu": 41, "cruiser_vram": 36,
		"cruiser_c": 27, "cruiser_kw": 18, "k3s_ok": 1,
		"starlink_snr": 8.4, "starlink_obstr": 2, "starlink_ms": 42,
		"sdwan_ok": 1, "lte_rsrp": -92, "p5g_ue": 12, "p5g_prb": 22,
		"drone_batt": 78, "drone_alt": 0, "drone_link": 1,
		"cam_fps": 18, "ppe_score": 96, "intrude": 0,
		"wx_c": 29, "flood_mm": 0, "gps_fix": 1, "fw_ready": 1,
	}
}

func (e *Engine) SetScenario(s string) Snapshot {
	e.mu.Lock()
	e.scenario = s
	e.mu.Unlock()
	return e.Tick()
}

func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.stop = make(chan struct{})
	iv := e.interval
	e.mu.Unlock()
	go func() {
		t := time.NewTicker(iv)
		defer t.Stop()
		if step := e.onStep; step != nil {
			e.mu.Lock()
			e.step()
			snap := e.snap()
			e.mu.Unlock()
			step(snap)
		} else {
			e.Tick()
		}
		for {
			select {
			case <-e.stop:
				return
			case <-t.C:
				e.mu.Lock()
				e.step()
				snap := e.snap()
				e.mu.Unlock()
				if step := e.onStep; step != nil {
					step(snap)
				}
			}
		}
	}()
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		e.running = false
		close(e.stop)
	}
}

func (e *Engine) Tick() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.step()
	return e.snap()
}

func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snap()
}

func (e *Engine) step() {
	v := e.values
	for k, n := range nominal() {
		v[k] = n + (rand.Float64()-0.5)*0.08*(1+n)
	}
	v["beacon_ok"], v["k3s_ok"], v["sdwan_ok"], v["drone_link"], v["gps_fix"], v["fw_ready"], v["intrude"] = 1, 1, 1, 1, 1, 1, 0
	v["drone_alt"] = 0
	switch e.scenario {
	case "sat_down":
		v["starlink_snr"] = 1.1
		v["starlink_obstr"] = 64
		v["starlink_ms"] = 420
		v["sdwan_ok"] = 0
	case "offline":
		v["starlink_snr"] = 0
		v["sdwan_ok"] = 0
		v["lte_rsrp"] = -128
		v["k3s_ok"] = 1 // local infer still up
	case "gpu_hot":
		v["cruiser_gpu"] = 96
		v["cruiser_vram"] = 91
		v["cruiser_c"] = 48
		v["cruiser_kw"] = 54
	case "drone_patrol":
		v["drone_alt"] = 42
		v["drone_batt"] = 15
		v["drone_link"] = 1
		v["cam_fps"] = 22
	case "intrusion":
		v["intrude"] = 1
		v["ppe_score"] = 41
		v["cam_fps"] = 24
		v["drone_alt"] = 28
	case "flood":
		v["flood_mm"] = 180
		v["fw_ready"] = 0
		v["gps_fix"] = 1
	case "p5g_load":
		v["p5g_ue"] = 48
		v["p5g_prb"] = 88
	}
}

func (e *Engine) snap() Snapshot {
	link := "starlink"
	if e.values["starlink_snr"] < 2 && e.values["lte_rsrp"] > -120 {
		link = "lte"
	}
	if e.values["sdwan_ok"] < 1 && e.values["starlink_snr"] < 2 && e.values["lte_rsrp"] <= -120 {
		link = "offline"
	}
	var rs []Reading
	for _, a := range Catalog() {
		val := e.values[a.Metric]
		sev := "info"
		switch a.Metric {
		case "starlink_snr":
			if val < 3 {
				sev = "critical"
			}
		case "cruiser_c":
			if val > 42 {
				sev = "warning"
			}
			if val > 50 {
				sev = "critical"
			}
		case "intrude", "flood_mm":
			if val > 0 && a.Metric == "intrude" || val > 50 && a.Metric == "flood_mm" {
				sev = "critical"
			}
		case "sdwan_ok", "k3s_ok", "beacon_ok", "fw_ready":
			if val < 1 {
				sev = "critical"
			}
		}
		rs = append(rs, Reading{
			ID: a.ID, Name: a.Name, Class: a.Class, Unit: a.Unit,
			Protocol: a.Protocol, Vendor: a.Vendor, Value: float64(int(val*100+0.5)) / 100, Severity: sev,
		})
	}
	return Snapshot{
		Scenario: e.scenario, Running: e.running, Readings: rs,
		Values: e.values, LinkMode: link, UpdatedAt: time.Now().UTC(),
	}
}

func Derive(s Snapshot) []Event {
	var out []Event
	ev := func(t, sev, cmd, dev string, data map[string]any) {
		out = append(out, Event{Type: t, Severity: sev, Command: cmd, DeviceID: dev, Data: data, At: time.Now().UTC()})
	}
	v := s.Values
	if v["starlink_snr"] < 3 {
		ev("remote-edge.link.starlink.degraded", "critical", "sdwan.failover", "gal_sat", map[string]any{"snr": v["starlink_snr"], "rtt_ms": v["starlink_ms"]})
	}
	if s.LinkMode == "offline" {
		ev("remote-edge.link.offline", "critical", "store_and_forward", "gal_beacon", map[string]any{"k3s_ok": v["k3s_ok"]})
	}
	if v["cruiser_c"] > 45 || v["cruiser_gpu"] > 92 {
		ev("remote-edge.galleon.thermal", "warning", "workload.shed", "gal_gpu", map[string]any{"c": v["cruiser_c"], "gpu": v["cruiser_gpu"]})
	}
	if v["intrude"] >= 1 {
		ev("remote-edge.vision.intrusion", "critical", "uav.launch", "cam_perim", map[string]any{"ppe": v["ppe_score"]})
	}
	if v["flood_mm"] > 50 {
		ev("remote-edge.iot.flood", "critical", "site.evacuate_low", "wx_flood", map[string]any{"mm": v["flood_mm"]})
	}
	if v["drone_batt"] < 20 && v["drone_alt"] > 5 {
		ev("remote-edge.uav.rtb", "warning", "uav.rtl", "uav_yard", map[string]any{"batt": v["drone_batt"]})
	}
	return out
}
