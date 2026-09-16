// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zyvorai/relay-edge/internal/relaypub"
)

func TestFwSeed_HappyPath(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/seed", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"site"`) || !strings.Contains(rr.Body.String(), `"season"`) {
		t.Fatalf("seed response missing expected fields: %s", rr.Body.String())
	}
}

func TestFwStartStopTick_HappyPath(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/start", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"running":true`) {
		t.Fatalf("start want 200 running:true, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/tick", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("tick want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/stop", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"running":false`) {
		t.Fatalf("stop want 200 running:false, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwScenario_MissingScenario(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/scenario", strings.NewReader(`{}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwScenario_HappyPath(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/scenario", strings.NewReader(`{"scenario":"fire"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
	var snap map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap["scenario"] != "fire" {
		t.Fatalf("scenario = %v, want fire", snap["scenario"])
	}
}

func TestFwConfig_MalformedJSON(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/config", strings.NewReader(`{"interval_ms":`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwConfig_HappyPath(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	body := `{"scenario":"lowtank","interval_ms":5000,"telemetry_always":true}`
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/config", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
	var cfg map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["scenario"] != "lowtank" || cfg["interval_ms"] != float64(5000) {
		t.Fatalf("config not applied: %+v", cfg)
	}
}
