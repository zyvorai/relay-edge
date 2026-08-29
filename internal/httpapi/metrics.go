// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type metrics struct {
	started   time.Time
	requests  atomic.Uint64
	errors    atomic.Uint64 // status >= 500
	publishes atomic.Uint64
}

func (s *Server) withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.metrics != nil {
			s.metrics.requests.Add(1)
		}
		rw := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(rw, r)
		if s.metrics != nil && rw.code >= 500 {
			s.metrics.errors.Add(1)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *Server) IncPublish() {
	if s.metrics != nil {
		s.metrics.publishes.Add(1)
	}
}

func (s *Server) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	m := s.metrics
	if m == nil {
		w.WriteHeader(404)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	uptime := time.Since(m.started).Seconds()
	_, _ = fmt.Fprintf(w, "# HELP relay_edge_up 1 if process is up\n# TYPE relay_edge_up gauge\nrelay_edge_up 1\n")
	_, _ = fmt.Fprintf(w, "# HELP relay_edge_uptime_seconds Process uptime\n# TYPE relay_edge_uptime_seconds gauge\nrelay_edge_uptime_seconds %.3f\n", uptime)
	_, _ = fmt.Fprintf(w, "# HELP relay_edge_http_requests_total HTTP requests\n# TYPE relay_edge_http_requests_total counter\nrelay_edge_http_requests_total %d\n", m.requests.Load())
	_, _ = fmt.Fprintf(w, "# HELP relay_edge_http_errors_total HTTP 5xx responses\n# TYPE relay_edge_http_errors_total counter\nrelay_edge_http_errors_total %d\n", m.errors.Load())
	_, _ = fmt.Fprintf(w, "# HELP relay_edge_publishes_total Stamped publishes attempted\n# TYPE relay_edge_publishes_total counter\nrelay_edge_publishes_total %d\n", m.publishes.Load())
}
