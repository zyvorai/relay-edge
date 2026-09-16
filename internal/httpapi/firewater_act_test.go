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

func TestFwAct_MalformedJSON(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/firewater/act", strings.NewReader(`{"command":`))
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed JSON, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwAct_WrongToken(t *testing.T) {
	dir := t.TempDir()
	seasons, _ := season.Open(filepath.Join(dir, "seasons.json"))
	sites, _ := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	devices, _ := device.Open(filepath.Join(dir, "devices.json"))
	contacts, _ := contact.Open(filepath.Join(dir, "contacts.json"))
	s := httpapi.New(seasons, sites, devices, contacts,
		&relaypub.Client{RelayBase: "http://127.0.0.1:18080"},
		nil,
		httpapi.Options{Version: "test", APIToken: "secret"},
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/firewater/act", strings.NewReader(`{"command":"pump.start"}`))
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for wrong token, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwAct_HappyPath(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/firewater/act", strings.NewReader(`{"command":"pump.start"}`))
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusConflict {
		t.Fatalf("want 200 (allowed) or 409 (inhibited), got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "allowed") {
		t.Fatalf("response missing decision fields: %s", rr.Body.String())
	}
}

func TestFwAct_MissingCommand(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/firewater/act", strings.NewReader(`{}`))
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing command, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwVerify_RequiresCommand(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/verify", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 without ?command=, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwVerify_HappyPath(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/verify?command=pump.start", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwSnapshotAndEvents(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/snapshot", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("snapshot want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/events", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("events want 200, got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"items"`) {
		t.Fatalf("events response missing items: %s", rr.Body.String())
	}
}
