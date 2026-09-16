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

func TestRemoteEdgeCatalogAndSnapshot(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/remote-edge/catalog", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"items"`) {
		t.Fatalf("catalog want 200 w/ items, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/remote-edge/snapshot", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"link_mode"`) {
		t.Fatalf("snapshot want 200 w/ link_mode, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestRemoteEdgeStartStopTick(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/remote-edge/start", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"running":true`) {
		t.Fatalf("start want 200 running:true, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/remote-edge/tick", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("tick want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/remote-edge/stop", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"running":false`) {
		t.Fatalf("stop want 200 running:false, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestRemoteEdgeScenario(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/remote-edge/scenario", strings.NewReader(`{"scenario":"intrusion"}`)))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"intrusion"`) {
		t.Fatalf("want 200 w/ intrusion, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestRemoteEdgeConfig_MalformedJSON(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/remote-edge/config", strings.NewReader(`{"publish":`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestRemoteEdgeConfig_HappyPathAndGet(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/remote-edge/config", strings.NewReader(`{"publish":true,"interval_ms":3000}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("post config want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/remote-edge/config", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"publish":true`) {
		t.Fatalf("get config want 200 publish:true, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestRemoteEdgeEvents(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/remote-edge/events", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"items"`) {
		t.Fatalf("want 200 w/ items, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestRemoteEdge_WrongToken(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, "/v1/remote-edge/start", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rr.Code, rr.Body.String())
	}
}
