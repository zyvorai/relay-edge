// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
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

func TestBodyLimitRejectsOversizePOST(t *testing.T) {
	dir := t.TempDir()
	seasons, _ := season.Open(filepath.Join(dir, "seasons.json"))
	sites, _ := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	devices, _ := device.Open(filepath.Join(dir, "devices.json"))
	contacts, _ := contact.Open(filepath.Join(dir, "contacts.json"))
	s := httpapi.New(seasons, sites, devices, contacts,
		&relaypub.Client{RelayBase: "http://127.0.0.1:18080"},
		nil,
		httpapi.Options{Version: "test", MaxBodyBytes: 64},
	)
	body := `{"id":"site_big","name":"` + strings.Repeat("x", 200) + `"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sites", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge && rr.Code != http.StatusBadRequest {
		t.Fatalf("want 413 or 400 for oversize body, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestBodyLimitAllowsSmallPOST(t *testing.T) {
	dir := t.TempDir()
	seasons, _ := season.Open(filepath.Join(dir, "seasons.json"))
	sites, _ := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	devices, _ := device.Open(filepath.Join(dir, "devices.json"))
	contacts, _ := contact.Open(filepath.Join(dir, "contacts.json"))
	s := httpapi.New(seasons, sites, devices, contacts,
		&relaypub.Client{RelayBase: "http://127.0.0.1:18080"},
		nil,
		httpapi.Options{Version: "test", MaxBodyBytes: 1 << 20},
	)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sites", strings.NewReader(`{"id":"site_ok","name":"OK"}`))
	req.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("small body should succeed, got %d %s", rr.Code, rr.Body.String())
	}
}
