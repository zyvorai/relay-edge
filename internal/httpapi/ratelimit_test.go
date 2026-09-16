// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/httpapi"
	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
)

func newRateLimitedServer(t *testing.T, rps float64, burst int) *httpapi.Server {
	t.Helper()
	dir := t.TempDir()
	seasons, _ := season.Open(filepath.Join(dir, "seasons.json"))
	sites, _ := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	devices, _ := device.Open(filepath.Join(dir, "devices.json"))
	contacts, _ := contact.Open(filepath.Join(dir, "contacts.json"))
	return httpapi.New(seasons, sites, devices, contacts,
		&relaypub.Client{RelayBase: "http://127.0.0.1:18080"},
		nil,
		httpapi.Options{Version: "test", RateLimitRPS: rps, RateLimitBurst: burst},
	)
}

func getSites(s *httpapi.Server) int {
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sites", nil))
	return rr.Code
}

func TestRateLimitRejectsBurstOverflow(t *testing.T) {
	s := newRateLimitedServer(t, 1, 2)
	if code := getSites(s); code != http.StatusOK {
		t.Fatalf("request 1: want 200, got %d", code)
	}
	if code := getSites(s); code != http.StatusOK {
		t.Fatalf("request 2: want 200, got %d", code)
	}
	if code := getSites(s); code != http.StatusTooManyRequests {
		t.Fatalf("request 3 (burst exceeded): want 429, got %d", code)
	}
}

func TestRateLimitRefillsOverTime(t *testing.T) {
	s := newRateLimitedServer(t, 1, 2)
	getSites(s)
	getSites(s)
	if code := getSites(s); code != http.StatusTooManyRequests {
		t.Fatalf("want 429 once burst is exhausted, got %d", code)
	}

	time.Sleep(1100 * time.Millisecond) // >1s at 1 token/sec refills at least one token

	if code := getSites(s); code != http.StatusOK {
		t.Fatalf("want 200 after refill, got %d", code)
	}
	if code := getSites(s); code != http.StatusTooManyRequests {
		t.Fatalf("want 429 immediately after the refilled token is spent, got %d", code)
	}
}

func TestRateLimitDisabledByDefault(t *testing.T) {
	s := newRateLimitedServer(t, 0, 0)
	for i := 0; i < 10; i++ {
		if code := getSites(s); code != http.StatusOK {
			t.Fatalf("request %d: rate limiting should be disabled when RateLimitRPS=0, got %d", i, code)
		}
	}
}

func TestRateLimitExemptsPublicGETs(t *testing.T) {
	s := newRateLimitedServer(t, 0.001, 1) // effectively one request ever, if not exempt
	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("healthz request %d: want 200 (public GETs exempt), got %d", i, rr.Code)
		}
	}
}
