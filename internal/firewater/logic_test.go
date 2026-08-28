// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater_test

import (
	"testing"
	"time"

	"github.com/zyvorai/relay-edge/internal/firewater"
)

func TestInhibitDryTank(t *testing.T) {
	v := map[string]float64{"tank_level": 12, "valve_open": 100, "plc_ok": 1, "lel_pct": 0}
	d := firewater.Evaluate("pump.start", v)
	if d.Allowed {
		t.Fatalf("expected inhibit, got %+v", d)
	}
}

func TestAllowPumpWhenHealthy(t *testing.T) {
	v := map[string]float64{"tank_level": 80, "valve_open": 100, "plc_ok": 1, "lel_pct": 0, "riser_bar": 3}
	d := firewater.Evaluate("pump.start", v)
	if !d.Allowed {
		t.Fatalf("expected allow: %+v", d)
	}
	if d.Verify == "" {
		t.Fatal("expected verify expression")
	}
}

func TestVerifyPump(t *testing.T) {
	v := map[string]float64{"header_lps": 12, "riser_bar": 8}
	got := firewater.Verify("pump.start", v)
	if got["verified"] != true {
		t.Fatalf("%v", got)
	}
}

func TestDownstream(t *testing.T) {
	ds := firewater.PlantGraph().Downstream("valve_a")
	if len(ds) == 0 {
		t.Fatal("expected riser downstream of valve")
	}
}

func TestAlarmAck(t *testing.T) {
	b := firewater.NewBook()
	b.Ingest([]firewater.Event{{Type: "firewater.tank.low", Severity: firewater.SevCritical, Command: "tank.fill"}})
	if len(b.List()) != 1 {
		t.Fatal("expected standing alarm")
	}
	b.Ack("firewater.tank.low")
	if b.List()[0].State != firewater.AlarmAcked {
		t.Fatalf("%s", b.List()[0].State)
	}
	b.Ingest(nil) // condition cleared
	b.Ack("firewater.tank.low")
	if len(b.List()) != 0 {
		t.Fatalf("expected cleared after return+ack: %+v", b.List())
	}
}

func TestWeeklyInhibit(t *testing.T) {
	got := firewater.RunWeeklyTest(map[string]float64{"tank_level": 10, "plc_ok": 1})
	if got.FailReason == "" {
		t.Fatal("expected fail reason")
	}
}

func TestSparkplug(t *testing.T) {
	msg := firewater.Sparkplug("g", "n", 1, map[string]float64{"tank_level": 80, "plc_ok": 1})
	if msg["topic"] != "spBv1.0/g/NDATA/n" {
		t.Fatalf("%v", msg["topic"])
	}
}

func TestShelve(t *testing.T) {
	b := firewater.NewBook()
	b.Ingest([]firewater.Event{{Type: "edge.comms.down", Severity: firewater.SevCritical}})
	a := b.Shelve("edge.comms.down", time.Minute)
	if a.State != firewater.AlarmShelved {
		t.Fatal(a.State)
	}
}
