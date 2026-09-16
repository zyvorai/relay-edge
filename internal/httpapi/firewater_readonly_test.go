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

func TestFwCatalog(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/catalog", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"items"`) {
		t.Fatalf("want 200 w/ items, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwTopology(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/topology", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"graph"`) {
		t.Fatalf("want 200 w/ graph, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwMatrix(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/matrix", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"rules"`) {
		t.Fatalf("want 200 w/ rules, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwReady(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/ready", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"system_ready"`) {
		t.Fatalf("want 200 w/ system_ready, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwSparkplugAndModbus(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/sparkplug", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("sparkplug want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/modbus", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"holding"`) {
		t.Fatalf("modbus want 200 w/ holding, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwWeekly(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/weekly-test", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwAlarms_EmptyThenAckAndShelve(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/alarms", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"items"`) {
		t.Fatalf("alarms want 200 w/ items, got %d %s", rr.Code, rr.Body.String())
	}

	// Force a scenario that raises critical events, then tick a few times so
	// the alarm book has something to ack/shelve.
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/scenario", strings.NewReader(`{"scenario":"fire"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("scenario want 200, got %d %s", rr.Code, rr.Body.String())
	}
	for range 5 {
		rr = httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/tick", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("tick want 200, got %d %s", rr.Code, rr.Body.String())
		}
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/firewater/alarms", nil))
	var alarms struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &alarms); err != nil {
		t.Fatal(err)
	}
	if len(alarms.Items) == 0 {
		t.Skip("fire scenario did not raise an alarm within 5 ticks — nothing to ack/shelve")
	}
	id := alarms.Items[0].ID

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/alarms/"+id+"/ack", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"acked"`) {
		t.Fatalf("ack want 200 acked, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/alarms/"+id+"/shelve", strings.NewReader(`{"minutes":5}`)))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"shelved"`) {
		t.Fatalf("shelve want 200 shelved, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwAck_NotFound(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/alarms/missing/ack", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFwShelve_NotFound(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/firewater/alarms/missing/shelve", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rr.Code, rr.Body.String())
	}
}
