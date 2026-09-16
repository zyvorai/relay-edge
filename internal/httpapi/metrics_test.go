// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/zyvorai/relay-edge/internal/relaypub"
)

// TestMetrics_MultipleServersDoNotPanic guards against a real gotcha: each
// Server has its own prometheus.Registry (not the global DefaultRegisterer),
// specifically so constructing many Servers in one process -- exactly what
// this test suite does across dozens of tests -- doesn't panic with
// "duplicate metrics collector registration attempted".
func TestMetrics_MultipleServersDoNotPanic(t *testing.T) {
	s1 := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	s2 := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	for _, s := range []interface{ Handler() http.Handler }{s1, s2} {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rr.Code)
		}
	}
}

func TestMetrics_PreservesExistingNames(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rr.Body.String()
	for _, name := range []string{
		"relay_edge_up",
		"relay_edge_uptime_seconds",
		"relay_edge_http_requests_total",
		"relay_edge_http_errors_total",
		"relay_edge_publishes_total",
	} {
		if !strings.Contains(body, name) {
			t.Fatalf("metrics output missing preserved series %q:\n%s", name, body)
		}
	}
}

func TestMetrics_NewHistogramAndRuntimeSeries(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})

	// Generate a request so the duration histogram has an observation.
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sites", nil))

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rr.Body.String()

	if !strings.Contains(body, "relay_edge_http_request_duration_seconds") {
		t.Fatalf("missing new histogram series:\n%s", body)
	}
	if !strings.Contains(body, `route="GET /v1/sites"`) {
		t.Fatalf("histogram missing bounded route label for a known route:\n%s", body)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Fatalf("missing Go runtime collector series:\n%s", body)
	}
	// client_golang's process collector only emits process_* series on
	// Linux (procfs-based) -- don't assert it on other dev platforms.
	if runtime.GOOS == "linux" && !strings.Contains(body, "process_") {
		t.Fatalf("missing process collector series on linux:\n%s", body)
	}
}

func TestMetrics_UnmatchedRouteIsBounded(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/no/such/path/at/all", nil))

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rr.Body.String()
	if !strings.Contains(body, `route="unmatched"`) {
		t.Fatalf("expected unmatched route to be bucketed as \"unmatched\", not the raw path:\n%s", body)
	}
	if strings.Contains(body, `route="/no/such/path/at/all"`) {
		t.Fatalf("raw unmatched path leaked into a metric label (unbounded cardinality risk):\n%s", body)
	}
}
