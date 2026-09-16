// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/firewater"
	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
)

func newSimPublishServer(t *testing.T, relayBase string) *Server {
	t.Helper()
	dir := t.TempDir()
	seasons, err := season.Open(filepath.Join(dir, "seasons.json"))
	if err != nil {
		t.Fatal(err)
	}
	sites, err := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	if err != nil {
		t.Fatal(err)
	}
	devices, err := device.Open(filepath.Join(dir, "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	contacts, err := contact.Open(filepath.Join(dir, "contacts.json"))
	if err != nil {
		t.Fatal(err)
	}
	return New(seasons, sites, devices, contacts, &relaypub.Client{RelayBase: relayBase}, nil, Options{Version: "test"})
}

func TestPublishSimEvent_NoSeasonDoesNotPublish(t *testing.T) {
	published := false
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		published = true
		w.WriteHeader(200)
	}))
	defer mock.Close()

	s := newSimPublishServer(t, mock.URL)
	// No firewater.Seed call — season_fw_watch does not exist yet.
	s.publishSimEvent("firewater.test", "info", "", "dev1", "firewater", nil)

	if published {
		t.Fatal("publishSimEvent should not call Relay when the season is missing")
	}
}

func TestPublishSimEvent_PublishesStampedEvent(t *testing.T) {
	var gotBody map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/events" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer mock.Close()

	s := newSimPublishServer(t, mock.URL)
	if _, err := firewater.Seed(s.Sites, s.Devices, s.Contacts, s.Seasons); err != nil {
		t.Fatal(err)
	}

	s.publishSimEvent("firewater.tank.low", "critical", "pump.start", "dev_fw_tank01", "firewater",
		map[string]any{"level_pct": 12})

	if gotBody == nil {
		t.Fatal("Relay never received a publish")
	}
	if gotBody["type"] != "firewater.tank.low" {
		t.Fatalf("type = %v, want firewater.tank.low", gotBody["type"])
	}
	if gotBody["severity"] != "critical" {
		t.Fatalf("severity = %v, want critical", gotBody["severity"])
	}
	data, _ := gotBody["data"].(map[string]any)
	if data == nil {
		t.Fatalf("data missing in payload: %+v", gotBody)
	}
	if data["sim_domain"] != "firewater" {
		t.Fatalf("sim_domain = %v, want firewater", data["sim_domain"])
	}
	action, _ := data["recommended_action"].(map[string]any)
	if action == nil || action["command"] != "pump.start" {
		t.Fatalf("recommended_action missing/wrong: %+v", data["recommended_action"])
	}
}
