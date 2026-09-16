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

func TestFleetCatalogAndSnapshot(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fleet/catalog", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"classes"`) {
		t.Fatalf("catalog want 200 w/ classes, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fleet/snapshot", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"scenario"`) {
		t.Fatalf("snapshot want 200 w/ scenario, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFleetStartStopTick(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fleet/start", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"running":true`) {
		t.Fatalf("start want 200 running:true, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fleet/tick", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("tick want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fleet/stop", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"running":false`) {
		t.Fatalf("stop want 200 running:false, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFleetScenario(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fleet/scenario", strings.NewReader(`{"scenario":"blackout"}`)))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"blackout"`) {
		t.Fatalf("want 200 w/ blackout, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFleetConfig_MalformedJSON(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fleet/config", strings.NewReader(`{"publish":`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFleetConfig_HappyPathAndGet(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fleet/config", strings.NewReader(`{"publish":true,"interval_ms":3000}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("post config want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fleet/config", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"publish":true`) {
		t.Fatalf("get config want 200 publish:true, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFleetEvents(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fleet/events", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"items"`) {
		t.Fatalf("want 200 w/ items, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFleet_WrongToken(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, "/v1/fleet/start", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rr.Code, rr.Body.String())
	}
}
