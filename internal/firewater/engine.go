// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater

import (
	"math/rand"
	"sync"
	"time"
)

const (
	SiteID    = "site_fw_plant"
	ZoneID    = "zone_fw_process_a"
	ZoneCode  = "FW-A"
	SeasonID  = "season_fw_watch"
	ContactID = "contact_fw_ehs"
	GatewayID = "dev_fw_gateway"
)

type Severity string

const (
	SevInfo     Severity = "info"
	SevWarning  Severity = "warning"
	SevCritical Severity = "critical"
)

type SensorDef struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Unit     string   `json:"unit"`
	Min      float64  `json:"min"`
	Max      float64  `json:"max"`
	Warn     *float64 `json:"warn,omitempty"`
	Crit     *float64 `json:"crit,omitempty"`
	Invert   bool     `json:"invert"`
	Device   string   `json:"device_id"`
	Kind     string   `json:"kind"`
	Class    string   `json:"class"`
	Protocol string   `json:"protocol"`
	Vendor   string   `json:"vendor"`
}

type Event struct {
	Type     string         `json:"type"`
	Severity Severity       `json:"severity"`
	Command  string         `json:"command,omitempty"`
	ZoneID   string         `json:"zone_id"`
	DeviceID string         `json:"device_id"`
	Data     map[string]any `json:"data"`
	At       time.Time      `json:"created_at"`
}

type Reading struct {
	Metric   string  `json:"metric"`
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Severity string  `json:"severity"`
	DeviceID string  `json:"device_id"`
	Class    string  `json:"class"`
	Protocol string  `json:"protocol"`
	Vendor   string  `json:"vendor"`
	Kind     string  `json:"kind"`
}

type Snapshot struct {
	Scenario    string             `json:"scenario"`
	Running     bool               `json:"running"`
	IntervalMS  int                `json:"interval_ms"`
	PumpOn      bool               `json:"pump_on"`
	SeasonID    string             `json:"season_id"`
	SiteID      string             `json:"site_id"`
	ZoneID      string             `json:"zone_id"`
	Readings    []Reading          `json:"readings"`
	Values      map[string]float64 `json:"values"`
	UpdatedAt   time.Time          `json:"updated_at"`
	SystemReady bool               `json:"system_ready"`
	ReadyWhy    []string           `json:"ready_why"`
	Alarms      []Alarm            `json:"alarms,omitempty"`
}

type Config struct {
	Scenario        string `json:"scenario"`
	IntervalMS      int    `json:"interval_ms"`
	TelemetryAlways bool   `json:"telemetry_always"`
	AlertsOnly      bool   `json:"alerts_only"`
	Publish         bool   `json:"publish"`
}

type Engine struct {
	mu        sync.Mutex
	scenario  string
	running   bool
	interval  time.Duration
	values    map[string]float64
	pumpOn    bool
	stop      chan struct{}
	onTick    func([]Event, Snapshot)
	telemetry bool
	alerts    bool
	publish   bool
	book      *Book
	seq       int
}

func New(on func([]Event, Snapshot)) *Engine {
	return &Engine{
		scenario:  "normal",
		interval:  2 * time.Second,
		values:    defaultValues(),
		onTick:    on,
		telemetry: true,
		book:      NewBook(),
	}
}

func (e *Engine) Book() *Book { return e.book }
func (e *Engine) Values() map[string]float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return clone(e.values)
}
func (e *Engine) NextSeq() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seq++
	return e.seq
}

func defaultValues() map[string]float64 {
	return map[string]float64{
		"tank_level": 86, "jockey_bar": 8.2, "main_bar": 0.4, "riser_bar": 7.6,
		"header_lps": 0.1, "valve_open": 100, "hydrant_bar": 7.1, "room_c": 24,
		"flood_mm": 0, "diesel_pct": 78, "pump_amps": 0, "tamper": 0,
		"pump_vib": 2.1, "leak_db": 38, "turbidity": 2.4,
		"deluge_open": 0, "foam_pct": 64, "heater_on": 0, "siren_on": 0, "vfd_hz": 18,
		"plc_ok": 1, "facp_ok": 1, "diesel_ecu": 1, "io_pack": 1,
		"edge_cpu": 22, "edge_temp": 48, "model_fps": 24, "k3s_ok": 1,
		"mqtt_ok": 1, "opcua_ok": 1, "lora_rssi": -78, "cell_rsrp": -85, "tsn_ok": 1, "ptp_lock": 1,
		"cam_ai_fire": 4, "cam_smoke": 3, "flame_ir": 2,
		"mains_ok": 1, "ups_soc": 96, "gen_run": 0, "pdu_w": 420,
		"lel_pct": 0.4, "co_ppm": 3, "smoke_pct": 0.8,
		"door_secure": 1, "nfc_lock": 1, "occupancy": 1,
	}
}

func (e *Engine) Config() Config {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Config{
		Scenario:        e.scenario,
		IntervalMS:      int(e.interval / time.Millisecond),
		TelemetryAlways: e.telemetry,
		AlertsOnly:      e.alerts,
		Publish:         e.publish,
	}
}

func (e *Engine) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Engine) PublishEnabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.publish
}

func (e *Engine) SetScenario(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.scenario = s
}

func (e *Engine) Apply(cfg Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cfg.Scenario != "" {
		e.scenario = cfg.Scenario
	}
	if cfg.IntervalMS >= 250 {
		e.interval = time.Duration(cfg.IntervalMS) * time.Millisecond
	}
	e.telemetry = cfg.TelemetryAlways
	e.alerts = cfg.AlertsOnly
	e.publish = cfg.Publish
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
		e.Tick()
		for {
			select {
			case <-e.stop:
				return
			case <-t.C:
				e.mu.Lock()
				if e.interval != iv {
					t.Reset(e.interval)
					iv = e.interval
				}
				e.mu.Unlock()
				e.Tick()
			}
		}
	}()
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	e.running = false
	close(e.stop)
}

func (e *Engine) Tick() Snapshot {
	e.mu.Lock()
	e.step()
	vals := clone(e.values)
	scen := e.scenario
	pump := e.pumpOn
	tel := e.telemetry
	alerts := e.alerts
	running := e.running
	iv := int(e.interval / time.Millisecond)
	e.mu.Unlock()

	evts := Derive(scen, vals, pump)
	if e.book != nil {
		e.book.Ingest(evts)
	}
	snap := snapshot(scen, running, iv, pump, vals)
	if e.book != nil {
		snap.Alarms = e.book.List()
	}
	if tel && !alerts {
		evts = append([]Event{Telemetry(vals, scen, pump)}, evts...)
	}
	if alerts {
		kept := evts[:0]
		for _, ev := range evts {
			if ev.Severity != SevInfo {
				kept = append(kept, ev)
			}
		}
		evts = kept
	}
	if e.onTick != nil {
		e.onTick(evts, snap)
	}
	return snap
}

func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	snap := snapshot(e.scenario, e.running, int(e.interval/time.Millisecond), e.pumpOn, clone(e.values))
	if e.book != nil {
		snap.Alarms = e.book.List()
	}
	return snap
}

func clone(m map[string]float64) map[string]float64 {
	o := make(map[string]float64, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}

func jitter(n, amp float64) float64 { return n + (rand.Float64()-0.5)*amp }

func (e *Engine) step() {
	v := e.values
	e.healthyFleet()
	switch e.scenario {
	case "fire":
		v["tank_level"] = maxf(35, v["tank_level"]-0.35)
		v["jockey_bar"] = jitter(7.0, 0.2)
		v["main_bar"] = jitter(9.4, 0.15)
		v["riser_bar"] = jitter(8.1, 0.12)
		v["header_lps"] = jitter(28, 1.4)
		v["valve_open"] = 100
		v["hydrant_bar"] = jitter(6.4, 0.2)
		v["room_c"] = jitter(31, 0.4)
		v["pump_amps"] = jitter(148, 4)
		v["pump_vib"] = jitter(6.8, 0.4)
		v["deluge_open"] = 100
		v["siren_on"] = 1
		v["vfd_hz"] = jitter(50, 1)
		v["cam_ai_fire"] = jitter(82, 4)
		v["cam_smoke"] = jitter(71, 5)
		v["flame_ir"] = jitter(77, 4)
		v["smoke_pct"] = jitter(28, 3)
		v["model_fps"] = jitter(18, 2)
		v["edge_cpu"] = jitter(74, 4)
		v["occupancy"] = jitter(6, 1)
		e.pumpOn = true
	case "lowtank":
		v["tank_level"] = maxf(18, jitter(22, 0.8))
	case "lowpress":
		v["jockey_bar"] = jitter(5.1, 0.15)
		v["riser_bar"] = jitter(3.6, 0.12)
		v["hydrant_bar"] = jitter(3.2, 0.1)
		v["header_lps"] = jitter(2.4, 0.3)
		v["vfd_hz"] = jitter(48, 2)
	case "pumpfail":
		v["header_lps"] = jitter(6, 0.4)
		v["riser_bar"] = jitter(3.2, 0.2)
		v["main_bar"] = jitter(0.6, 0.1)
		v["pump_amps"] = jitter(8, 2)
		v["pump_vib"] = jitter(0.4, 0.1)
		v["diesel_ecu"] = 0
		v["plc_ok"] = 1
		e.pumpOn = false
	case "valve":
		v["valve_open"] = 0
		v["nfc_lock"] = 1
	case "leak":
		v["header_lps"] = jitter(1.8, 0.2)
		v["jockey_bar"] = jitter(6.2, 0.15)
		v["tank_level"] = maxf(40, v["tank_level"]-0.08)
		v["leak_db"] = jitter(81, 3)
	case "freeze":
		v["room_c"] = jitter(2.4, 0.3)
		v["heater_on"] = 1
	case "hydrant":
		v["tamper"] = 1
		v["hydrant_bar"] = jitter(0.4, 0.1)
		v["nfc_lock"] = 0
		v["door_secure"] = 0
	case "comms":
		v["mqtt_ok"] = 0
		v["opcua_ok"] = 0
		v["lora_rssi"] = jitter(-122, 2)
		v["cell_rsrp"] = jitter(-124, 2)
		v["tsn_ok"] = 0
		v["ptp_lock"] = 0
		v["k3s_ok"] = 0
	case "vision":
		v["cam_ai_fire"] = jitter(88, 3)
		v["cam_smoke"] = jitter(64, 4)
		v["flame_ir"] = jitter(73, 3)
		v["model_fps"] = jitter(9, 1)
		v["edge_cpu"] = jitter(88, 3)
		v["edge_temp"] = jitter(79, 2)
	case "power":
		v["mains_ok"] = 0
		v["ups_soc"] = jitter(18, 2)
		v["gen_run"] = 1
		v["pdu_w"] = jitter(3100, 80)
		v["plc_ok"] = 1
	case "gas":
		v["lel_pct"] = jitter(24, 2)
		v["co_ppm"] = jitter(62, 4)
		v["siren_on"] = 1
		v["occupancy"] = 0
	case "plc":
		v["plc_ok"] = 0
		v["io_pack"] = 0
		v["facp_ok"] = 0
	default:
		e.pumpOn = false
	}
}

func (e *Engine) healthyFleet() {
	v := e.values
	base := defaultValues()
	for k, n := range base {
		amp := 0.04 * (1 + n)
		if n <= 1 && n >= 0 && (k == "plc_ok" || k == "facp_ok" || k == "diesel_ecu" || k == "io_pack" ||
			k == "mqtt_ok" || k == "opcua_ok" || k == "tsn_ok" || k == "ptp_lock" || k == "k3s_ok" ||
			k == "mains_ok" || k == "door_secure" || k == "nfc_lock" || k == "heater_on" || k == "siren_on" ||
			k == "gen_run" || k == "tamper") {
			v[k] = n
			continue
		}
		v[k] = jitter(n, amp)
	}
	v["tank_level"] = jitter(86, 0.6)
	v["jockey_bar"] = jitter(8.2, 0.08)
	v["main_bar"] = jitter(0.4, 0.05)
	v["riser_bar"] = jitter(7.6, 0.08)
	v["header_lps"] = jitter(0.08, 0.04)
	v["valve_open"] = 100
	v["hydrant_bar"] = jitter(7.1, 0.06)
	v["room_c"] = jitter(24, 0.3)
	v["flood_mm"] = 0
	v["diesel_pct"] = jitter(78, 0.1)
	v["pump_amps"] = 0
	v["tamper"] = 0
}

func snapshot(scen string, running bool, iv int, pump bool, vals map[string]float64) Snapshot {
	var readings []Reading
	for _, s := range Catalog() {
		val := vals[s.ID]
		readings = append(readings, Reading{
			Metric: s.ID, Name: s.Name, Value: round(val), Unit: s.Unit,
			Min: s.Min, Max: s.Max, Severity: severity(s, val, scen), DeviceID: s.Device,
			Class: s.Class, Protocol: s.Protocol, Vendor: s.Vendor, Kind: s.Kind,
		})
	}
	return Snapshot{
		Scenario: scen, Running: running, IntervalMS: iv, PumpOn: pump,
		SeasonID: SeasonID, SiteID: SiteID, ZoneID: ZoneID,
		Readings: readings, Values: vals, UpdatedAt: time.Now().UTC(),
		SystemReady: SystemReady(vals), ReadyWhy: ReadyWhy(vals),
	}
}

func severity(s SensorDef, value float64, scen string) string {
	if s.ID == "tamper" {
		if value >= 1 {
			return string(SevCritical)
		}
		return string(SevInfo)
	}
	if s.ID == "header_lps" {
		if scen == "fire" {
			return string(SevCritical)
		}
		if value > 1.2 {
			return string(SevWarning)
		}
		return string(SevInfo)
	}
	if s.Crit == nil {
		return string(SevInfo)
	}
	if s.Invert {
		if value <= *s.Crit {
			return string(SevCritical)
		}
		if s.Warn != nil && value <= *s.Warn {
			return string(SevWarning)
		}
		return string(SevInfo)
	}
	if value <= *s.Crit {
		return string(SevCritical)
	}
	if s.Warn != nil && value <= *s.Warn {
		return string(SevWarning)
	}
	return string(SevInfo)
}

func ev(typ string, sev Severity, cmd, dev string, data map[string]any) Event {
	if data == nil {
		data = map[string]any{}
	}
	data["ts"] = time.Now().UTC().Format(time.RFC3339)
	data["domain"] = "industrial_firewater"
	return Event{
		Type: typ, Severity: sev, Command: cmd,
		ZoneID: ZoneID, DeviceID: dev, Data: data, At: time.Now().UTC(),
	}
}

func Telemetry(vals map[string]float64, scen string, pump bool) Event {
	readings := map[string]any{}
	for k, v := range vals {
		readings[k] = round(v)
	}
	readings["pump_on"] = pump
	return ev("telemetry.sample", SevInfo, "", GatewayID, map[string]any{
		"readings": readings, "scenario": scen,
	})
}

func Derive(scen string, v map[string]float64, pump bool) []Event {
	var out []Event
	if v["tank_level"] <= 30 {
		out = append(out, ev("firewater.tank.low", SevCritical, "tank.fill", "dev_fw_tank_lt01", map[string]any{"level_pct": round(v["tank_level"]), "limit_pct": 30}))
	} else if v["tank_level"] <= 50 {
		out = append(out, ev("firewater.tank.low", SevWarning, "tank.inspect", "dev_fw_tank_lt01", map[string]any{"level_pct": round(v["tank_level"]), "limit_pct": 50}))
	}
	if v["riser_bar"] <= 4.0 {
		out = append(out, ev("firewater.pressure.low", SevCritical, "pump.start", "dev_fw_riser_pt01", map[string]any{"pressure_bar": round(v["riser_bar"]), "point": "most_remote"}))
	} else if v["jockey_bar"] <= 5.5 {
		out = append(out, ev("firewater.pressure.low", SevWarning, "jockey.inspect", "dev_fw_jockey_pt01", map[string]any{"pressure_bar": round(v["jockey_bar"])}))
	}
	if scen == "fire" {
		out = append(out, ev("firewater.demand.active", SevCritical, "pump.confirm_run", "dev_fw_flow_ft01", map[string]any{"flow_lps": round(v["header_lps"]), "pump_on": pump}))
	}
	if scen == "pumpfail" {
		out = append(out, ev("firewater.pump.fail", SevCritical, "pump.start_backup", "dev_fw_pump_ct01", map[string]any{"amps": round(v["pump_amps"]), "expected_run": 1}))
	}
	if v["valve_open"] < 20 {
		out = append(out, ev("firewater.valve.closed", SevCritical, "valve.open", "dev_fw_valve_zs01", map[string]any{"open_pct": v["valve_open"], "tamper": 1}))
	}
	if v["header_lps"] > 1.2 && scen != "fire" {
		out = append(out, ev("firewater.flow.detected", SevWarning, "zone.investigate", "dev_fw_flow_ft01", map[string]any{"flow_lps": round(v["header_lps"]), "unexpected": true}))
	}
	if v["room_c"] <= 4 {
		out = append(out, ev("firewater.freeze.risk", SevCritical, "room.heat", "dev_fw_room_tt01", map[string]any{"temp_c": round(v["room_c"])}))
	}
	if v["tamper"] >= 1 {
		out = append(out, ev("firewater.hydrant.tamper", SevCritical, "security.dispatch", "dev_fw_hydrant_ts01", map[string]any{"pressure_bar": round(v["hydrant_bar"])}))
	}
	if v["flood_mm"] >= 30 {
		out = append(out, ev("firewater.pumproom.flood", SevCritical, "sump.start", "dev_fw_room_lt02", map[string]any{"level_mm": round(v["flood_mm"])}))
	}
	if v["diesel_pct"] <= 20 {
		out = append(out, ev("firewater.diesel.low", SevCritical, "fuel.refill", "dev_fw_diesel_lt01", map[string]any{"level_pct": round(v["diesel_pct"])}))
	}
	if v["leak_db"] >= 75 {
		out = append(out, ev("firewater.leak.acoustic", SevCritical, "zone.investigate", "dev_fw_leak_ae01", map[string]any{"level_db": round(v["leak_db"])}))
	}
	if v["cam_ai_fire"] >= 70 {
		out = append(out, ev("edge.vision.fire", SevCritical, "nvr.clip", "dev_fw_thermal_cam", map[string]any{"score_pct": round(v["cam_ai_fire"]), "model": "thermal-yolo"}))
	}
	if v["mqtt_ok"] < 1 || v["cell_rsrp"] <= -120 {
		out = append(out, ev("edge.comms.down", SevCritical, "radio.failover", "dev_fw_5g_cpe", map[string]any{"mqtt_ok": v["mqtt_ok"], "rsrp": round(v["cell_rsrp"])}))
	}
	if v["mains_ok"] < 1 || v["ups_soc"] <= 20 {
		out = append(out, ev("edge.power.fail", SevCritical, "genset.start", "dev_fw_ups", map[string]any{"mains_ok": v["mains_ok"], "ups_soc": round(v["ups_soc"]), "gen_run": v["gen_run"]}))
	}
	if v["lel_pct"] >= 20 || v["co_ppm"] >= 50 {
		out = append(out, ev("edge.gas.alarm", SevCritical, "siren.on", "dev_fw_gas_lel", map[string]any{"lel_pct": round(v["lel_pct"]), "co_ppm": round(v["co_ppm"])}))
	}
	if v["plc_ok"] < 1 || v["facp_ok"] < 1 {
		out = append(out, ev("edge.control.fault", SevCritical, "plc.reset", "dev_fw_plc_s7", map[string]any{"plc_ok": v["plc_ok"], "facp_ok": v["facp_ok"]}))
	}
	if v["nfc_lock"] < 1 || v["door_secure"] < 1 {
		out = append(out, ev("edge.access.breach", SevCritical, "security.dispatch", "dev_fw_door", map[string]any{"nfc_lock": v["nfc_lock"], "door_secure": v["door_secure"]}))
	}
	if v["k3s_ok"] < 1 {
		out = append(out, ev("edge.runtime.down", SevWarning, "k3s.recover", "dev_fw_k3s", map[string]any{"k3s_ok": 0}))
	}
	if v["pump_vib"] >= 11 {
		out = append(out, ev("firewater.pump.vibration", SevCritical, "pump.inspect", "dev_fw_pump_vt01", map[string]any{"mm_s": round(v["pump_vib"])}))
	}
	return out
}

func round(n float64) float64 { return float64(int(n*100+0.5)) / 100 }

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
