// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater

import "time"

// WeeklyTest is an NFPA-25-style pump churn record.
type WeeklyTest struct {
	ID          string         `json:"id"`
	Standard    string         `json:"standard"`
	StartedAt   time.Time      `json:"started_at"`
	DurationS   int            `json:"duration_s"`
	StartOK     bool           `json:"start_ok"`
	ChurnBar    float64        `json:"churn_bar"`
	FlowLPS     float64        `json:"flow_lps"`
	FailReason  string         `json:"fail_reason,omitempty"`
	SystemReady bool           `json:"system_ready"`
	Readings    map[string]any `json:"readings"`
}

func RunWeeklyTest(v map[string]float64) WeeklyTest {
	t := WeeklyTest{
		ID:        "nfpa25-weekly-" + time.Now().UTC().Format("20060102T150405"),
		Standard:  "NFPA 25 / TAC pump weekly churn",
		StartedAt: time.Now().UTC(),
		DurationS: 10,
		Readings: map[string]any{
			"tank_level": v["tank_level"],
			"riser_bar":  v["riser_bar"],
			"header_lps": v["header_lps"],
			"pump_amps":  v["pump_amps"],
			"diesel_ecu": v["diesel_ecu"],
			"plc_ok":     v["plc_ok"],
		},
		SystemReady: SystemReady(v),
	}
	if v["tank_level"] < 20 {
		t.FailReason = "inhibit: tank too low"
		return t
	}
	if v["plc_ok"] < 1 {
		t.FailReason = "inhibit: PLC down"
		return t
	}
	t.StartOK = v["pump_amps"] > 20 || v["header_lps"] > 5 || v["main_bar"] > 4
	t.ChurnBar = v["main_bar"]
	t.FlowLPS = v["header_lps"]
	if !t.StartOK {
		t.FailReason = "fail to start — no amps or flow"
	}
	return t
}
