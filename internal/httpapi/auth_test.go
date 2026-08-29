// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
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

func TestAPITokenAuth(t *testing.T) {
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
	s := httpapi.New(seasons, sites, devices, contacts, &relaypub.Client{RelayBase: "http://127.0.0.1:9"}, nil, httpapi.Options{
		Version:  "test-ver",
		APIToken: "secret-lab",
	})
	h := s.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("healthz should be public, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"auth_required":true`) {
		t.Fatalf("healthz missing auth_required: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != 200 {
		t.Fatalf("metrics should be public, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "relay_edge_up") {
		t.Fatalf("metrics body: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sites", nil))
	if rr.Code != 401 {
		t.Fatalf("sites without token want 401, got %d %s", rr.Code, rr.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sites", nil)
	req.Header.Set("Authorization", "Bearer secret-lab")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("sites with bearer want 200, got %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/config", nil)
	req.Header.Set("X-Edge-Token", "secret-lab")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("admin with X-Edge-Token want 200, got %d", rr.Code)
	}
}
