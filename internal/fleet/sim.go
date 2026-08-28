// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package fleet

import (
	"math/rand"
	"sync"
	"time"
)

type Reading struct {
	ID, Name, Class, Domain, Protocol, Unit, Severity string
	Value                                             float64
}

type Event struct {
	Type, Severity, Command, DeviceID string
	Data                              map[string]any
	At                                time.Time
}

type Snapshot struct {
	Scenario  string             `json:"scenario"`
	Running   bool               `json:"running"`
	Classes   []string           `json:"classes"`
	Readings  []Reading          `json:"readings"`
	Values    map[string]float64 `json:"values"`
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
	v := map[string]float64{}
	for _, d := range Catalog() {
		v[d.ID] = d.Nominal
	}
	return &Engine{scenario: "nominal", values: v, interval: 2 * time.Second, onStep: onStep}
}

func (e *Engine) SetInterval(ms int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ms >= 250 {
		e.interval = time.Duration(ms) * time.Millisecond
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
	for _, d := range Catalog() {
		n := d.Nominal
		if d.Flag {
			e.values[d.ID] = n
			continue
		}
		e.values[d.ID] = n + (rand.Float64()-0.5)*0.06*(1+n)
	}
	v := e.values
	switch e.scenario {
	case "blackout":
		v["inverter_ok"], v["rmu_ok"], v["wifi_ap"] = 0, 0, 0
		v["bess_soc"] = 18
		v["pv_kw"] = 0
	case "intrusion":
		v["anpr_ok"], v["face_gate"] = 1, 0
		v["bodycam"] = 1
		v["ids_evt"] = 12
	case "spill":
		v["ww_cl2"] = 0.05
		v["cems_nox"] = 210
		v["aqi_pm"] = 160
	case "amr_lost":
		v["amr_pose"] = 0
		v["amr_batt"] = 9
		v["uwb_anchor"] = 0
	case "ot_storm":
		v["mb_gw"], v["opc_gw"], v["pn_gw"] = 0, 0, 1
		v["ids_evt"] = 40
	case "heatwave":
		v["crac_c"] = 31
		v["bess_c"] = 48
		v["xfmr_c"] = 98
		v["reefer_c"] = -8
	case "flood":
		v["ww_inf"] = 720
		v["elight"] = 1
		v["refuge"] = 1
	}
}

func (e *Engine) snap() Snapshot {
	var rs []Reading
	for _, d := range Catalog() {
		val := e.values[d.ID]
		sev := "info"
		if d.Flag && val < 1 && d.Nominal >= 1 {
			sev = "critical"
		}
		if !d.Flag && d.Max > 0 && val > d.Nominal*1.6 && val > d.Nominal+5 {
			sev = "warning"
		}
		if d.ID == "amr_pose" && val < 1 {
			sev = "critical"
		}
		if d.ID == "bess_soc" && val < 20 {
			sev = "critical"
		}
		rs = append(rs, Reading{
			ID: d.ID, Name: d.Name, Class: d.Class, Domain: d.Domain,
			Protocol: d.Protocol, Unit: d.Unit, Value: float64(int(val*100+0.5)) / 100, Severity: sev,
		})
	}
	return Snapshot{Scenario: e.scenario, Running: e.running, Classes: Classes(), Readings: rs, Values: e.values, UpdatedAt: time.Now().UTC()}
}

func Derive(s Snapshot) []Event {
	var out []Event
	add := func(t, sev, cmd, id string, data map[string]any) {
		out = append(out, Event{Type: t, Severity: sev, Command: cmd, DeviceID: id, Data: data, At: time.Now().UTC()})
	}
	v := s.Values
	if v["inverter_ok"] < 1 || v["rmu_ok"] < 1 {
		add("fleet.power.island", "critical", "bess.discharge", "rmu_ok", map[string]any{"bess_soc": v["bess_soc"]})
	}
	if v["amr_pose"] < 1 {
		add("fleet.robot.lost", "critical", "amr.relocalize", "amr_pose", map[string]any{"batt": v["amr_batt"]})
	}
	if v["ids_evt"] > 8 {
		add("fleet.ot.ids", "warning", "ot.segment", "ids_evt", map[string]any{"n": v["ids_evt"]})
	}
	if v["cems_nox"] > 150 || v["aqi_pm"] > 100 {
		add("fleet.env.exceedance", "critical", "process.curtail", "cems_nox", map[string]any{"nox": v["cems_nox"], "pm": v["aqi_pm"]})
	}
	if v["crac_c"] > 28 {
		add("fleet.dc.thermal", "warning", "workload.shed", "crac_c", map[string]any{"c": v["crac_c"]})
	}
	if v["face_gate"] < 1 {
		add("fleet.access.fault", "critical", "security.lockdown", "face_gate", nil)
	}
	return out
}
