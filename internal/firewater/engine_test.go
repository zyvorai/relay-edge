// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/firewater"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
)

func TestFireScenarioRaisesDemand(t *testing.T) {
	var got []firewater.Event
	e := firewater.New(func(evts []firewater.Event, _ firewater.Snapshot) {
		got = append(got, evts...)
	})
	e.SetScenario("fire")
	snap := e.Tick()
	if !snap.PumpOn {
		t.Fatal("expected pump on")
	}
	found := false
	for _, ev := range got {
		if ev.Type == "firewater.demand.active" && ev.Severity == firewater.SevCritical {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing demand event: %+v", got)
	}
}

func TestValveScenario(t *testing.T) {
	e := firewater.New(nil)
	e.SetScenario("valve")
	snap := e.Tick()
	if snap.Values["valve_open"] >= 20 {
		t.Fatalf("valve should be shut, got %v", snap.Values["valve_open"])
	}
	evts := firewater.Derive("valve", snap.Values, false)
	ok := false
	for _, ev := range evts {
		if ev.Type == "firewater.valve.closed" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected valve.closed, got %+v", evts)
	}
}

func TestStartStop(t *testing.T) {
	e := firewater.New(nil)
	e.Apply(firewater.Config{IntervalMS: 250, Scenario: "normal"})
	e.Start()
	time.Sleep(300 * time.Millisecond)
	if !e.Running() {
		t.Fatal("expected running")
	}
	e.Stop()
	time.Sleep(40 * time.Millisecond)
	if e.Running() {
		t.Fatal("expected stopped")
	}
}

func TestSeedInventory(t *testing.T) {
	dir := t.TempDir()
	sites, err := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	if err != nil {
		t.Fatal(err)
	}
	devs, err := device.Open(filepath.Join(dir, "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	contacts, err := contact.Open(filepath.Join(dir, "contacts.json"))
	if err != nil {
		t.Fatal(err)
	}
	seasons, err := season.Open(filepath.Join(dir, "seasons.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := firewater.Seed(sites, devs, contacts, seasons)
	if err != nil {
		t.Fatal(err)
	}
	if got.Season.Status != "active" {
		t.Fatalf("season status %s", got.Season.Status)
	}
	if got.Zone.Code != firewater.ZoneCode {
		t.Fatalf("zone code %s", got.Zone.Code)
	}
	if len(got.Devices) < 10 {
		t.Fatalf("devices %d", len(got.Devices))
	}
}
