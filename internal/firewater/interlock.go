// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater

import "fmt"

// Rule is one row of the cause-and-effect matrix.
type Rule struct {
	ID       string `json:"id"`
	Command  string `json:"command"`
	When     string `json:"when"`
	Inhibit  bool   `json:"inhibit"`
	Reason   string `json:"reason,omitempty"`
	Act      string `json:"act,omitempty"`
	Verify   string `json:"verify,omitempty"`
	ProbeURL string `json:"probe_url,omitempty"`
}

type Decision struct {
	Command  string   `json:"command"`
	Allowed  bool     `json:"allowed"`
	Reasons  []string `json:"reasons"`
	Act      string   `json:"act,omitempty"`
	Verify   string   `json:"verify,omitempty"`
	ProbeURL string   `json:"probe_url,omitempty"`
	Ready    bool     `json:"system_ready"`
}

func Matrix() []Rule {
	return []Rule{
		{ID: "inh-tank", Command: "pump.start", When: "tank_level < 20", Inhibit: true, Reason: "tank below 20% — dry-run inhibit"},
		{ID: "inh-valve", Command: "pump.start", When: "valve_open < 20", Inhibit: true, Reason: "zone OS&Y shut — riser will not charge"},
		{ID: "inh-plc", Command: "pump.start", When: "plc_ok < 1", Inhibit: true, Reason: "PLC heartbeat lost"},
		{ID: "inh-lel", Command: "pump.start", When: "lel_pct >= 20", Inhibit: true, Reason: "pump-room LEL high — isolate ignition sources"},
		{ID: "act-pump", Command: "pump.start", When: "riser_bar < 4", Inhibit: false, Act: "start main + confirm jockey", Verify: "header_lps > 5 AND riser_bar > 6", ProbeURL: "/v1/firewater/verify?command=pump.start"},
		{ID: "act-backup", Command: "pump.start_backup", When: "diesel_ecu < 1", Inhibit: true, Reason: "diesel ECU not ready"},
		{ID: "act-valve", Command: "valve.open", When: "valve_open < 20", Inhibit: false, Act: "drive OS&Y open", Verify: "valve_open >= 90", ProbeURL: "/v1/firewater/verify?command=valve.open"},
		{ID: "act-heat", Command: "room.heat", When: "room_c <= 4", Inhibit: false, Act: "energize heater", Verify: "heater_on == 1", ProbeURL: "/v1/firewater/verify?command=room.heat"},
		{ID: "act-gen", Command: "genset.start", When: "mains_ok < 1", Inhibit: false, Act: "start standby genset", Verify: "gen_run == 1", ProbeURL: "/v1/firewater/verify?command=genset.start"},
		{ID: "act-siren", Command: "siren.on", When: "cam_ai_fire >= 70 OR lel_pct >= 20", Inhibit: false, Act: "yard siren + strobe", Verify: "siren_on == 1", ProbeURL: "/v1/firewater/verify?command=siren.on"},
	}
}

func Evaluate(cmd string, v map[string]float64) Decision {
	d := Decision{Command: cmd, Allowed: true, Ready: SystemReady(v)}
	for _, r := range Matrix() {
		if r.Command != cmd {
			continue
		}
		if !whenTrue(r.When, v) {
			continue
		}
		if r.Inhibit {
			d.Allowed = false
			d.Reasons = append(d.Reasons, r.Reason)
			continue
		}
		d.Act = r.Act
		d.Verify = r.Verify
		d.ProbeURL = r.ProbeURL
	}
	if !d.Allowed && d.Act == "" {
		d.Act = "inhibit"
	}
	return d
}

func SystemReady(v map[string]float64) bool {
	return v["tank_level"] >= 50 &&
		v["valve_open"] >= 90 &&
		v["plc_ok"] >= 1 &&
		v["facp_ok"] >= 1 &&
		v["mains_ok"]+v["gen_run"] >= 1 &&
		v["nfc_lock"] >= 1 &&
		v["lel_pct"] < 10
}

func ReadyWhy(v map[string]float64) []string {
	var why []string
	if v["tank_level"] < 50 {
		why = append(why, fmt.Sprintf("tank %.0f%% < 50%%", v["tank_level"]))
	}
	if v["valve_open"] < 90 {
		why = append(why, "zone valve not fully open")
	}
	if v["plc_ok"] < 1 {
		why = append(why, "PLC down")
	}
	if v["facp_ok"] < 1 {
		why = append(why, "FACP down")
	}
	if v["mains_ok"]+v["gen_run"] < 1 {
		why = append(why, "no utility and genset not running")
	}
	if v["nfc_lock"] < 1 {
		why = append(why, "hydrant lock open")
	}
	if v["lel_pct"] >= 10 {
		why = append(why, "LEL elevated")
	}
	if len(why) == 0 {
		why = append(why, "system ready — standing pressure available")
	}
	return why
}

func Verify(cmd string, v map[string]float64) map[string]any {
	ok := false
	detail := ""
	switch cmd {
	case "pump.start":
		ok = v["header_lps"] > 5 && v["riser_bar"] > 6
		detail = fmt.Sprintf("flow=%.1f L/s riser=%.2f bar", v["header_lps"], v["riser_bar"])
	case "valve.open":
		ok = v["valve_open"] >= 90
		detail = fmt.Sprintf("open=%.0f%%", v["valve_open"])
	case "room.heat":
		ok = v["heater_on"] >= 1
		detail = "heater"
	case "genset.start":
		ok = v["gen_run"] >= 1
		detail = "genset"
	case "siren.on":
		ok = v["siren_on"] >= 1
		detail = "siren"
	default:
		detail = "no probe"
	}
	return map[string]any{"command": cmd, "verified": ok, "detail": detail, "system_ready": SystemReady(v)}
}

func whenTrue(expr string, v map[string]float64) bool {
	// Tiny expression language: "metric OP number" plus OR.
	if expr == "" {
		return true
	}
	parts := splitOR(expr)
	for _, p := range parts {
		if cmp(p, v) {
			return true
		}
	}
	return false
}

func splitOR(s string) []string {
	var out []string
	cur := ""
	for i := 0; i < len(s); i++ {
		if i+4 <= len(s) && s[i:i+4] == " OR " {
			out = append(out, cur)
			cur = ""
			i += 3
			continue
		}
		cur += string(s[i])
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func cmp(expr string, v map[string]float64) bool {
	var metric, op string
	var num float64
	n, _ := fmt.Sscanf(expr, "%s %s %f", &metric, &op, &num)
	if n < 3 {
		return false
	}
	got := v[metric]
	switch op {
	case "<":
		return got < num
	case "<=":
		return got <= num
	case ">":
		return got > num
	case ">=":
		return got >= num
	case "==":
		return got == num
	}
	return false
}
