// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/httpapi"
	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
)

func testServer(t *testing.T, families []string, pub *relaypub.Client) *httpapi.Server {
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
	if pub == nil {
		pub = &relaypub.Client{RelayBase: "http://127.0.0.1:9"}
	}
	return httpapi.New(seasons, sites, devices, contacts, pub, families, "test-ver")
}

func moduleList(health map[string]any) string {
	mods, _ := health["modules"].([]any)
	parts := make([]string, 0, len(mods))
	for _, m := range mods {
		parts = append(parts, m.(string))
	}
	return "," + strings.Join(parts, ",") + ","
}

func TestHealthVersionReady(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	h := s.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("healthz status %d", rr.Code)
	}
	var health map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if health["version"] != "test-ver" {
		t.Fatalf("version: %#v", health["version"])
	}
	joined := moduleList(health)
	for _, need := range []string{"firewater", "remote-edge", "fleet", "ui"} {
		if !strings.Contains(joined, ","+need+",") {
			t.Fatalf("expected module %s in %s", need, joined)
		}
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/version", nil))
	if rr.Code != 200 {
		t.Fatalf("version status %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != 200 {
		t.Fatalf("readyz status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestReadyRequiresPublishTarget(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != 503 {
		t.Fatalf("want 503, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestEnabledFamiliesGate(t *testing.T) {
	s := testServer(t, []string{"fleet"}, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var health map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &health)
	joined := moduleList(health)
	if !strings.Contains(joined, ",fleet,") {
		t.Fatalf("fleet missing: %s", joined)
	}
	if strings.Contains(joined, ",firewater,") || strings.Contains(joined, ",remote-edge,") {
		t.Fatalf("unexpected families: %s", joined)
	}
}

func TestSiteCRUD(t *testing.T) {
	s := testServer(t, []string{}, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	h := s.Handler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sites", strings.NewReader(`{"id":"site_t","name":"Test Site"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != 200 && rr.Code != 201 {
		t.Fatalf("create site %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sites/site_t", nil))
	if rr.Code != 200 {
		t.Fatalf("get site %d %s", rr.Code, rr.Body.String())
	}
}
