// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics holds a per-Server Prometheus registry. It is deliberately NOT
// the global prometheus.DefaultRegisterer: internal/httpapi.New() is
// called once per test across many tests in the same process, and
// registering collectors globally would panic on the second call
// ("duplicate metrics collector registration attempted").
type metrics struct {
	registry  *prometheus.Registry
	up        prometheus.Gauge
	requests  prometheus.Counter
	errors    prometheus.Counter // status >= 500
	publishes prometheus.Counter
	duration  *prometheus.HistogramVec
}

func newMetrics() *metrics {
	started := time.Now()
	reg := prometheus.NewRegistry()

	m := &metrics{
		registry: reg,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "relay_edge_up",
			Help: "1 if process is up",
		}),
		requests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "relay_edge_http_requests_total",
			Help: "HTTP requests",
		}),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "relay_edge_http_errors_total",
			Help: "HTTP 5xx responses",
		}),
		publishes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "relay_edge_publishes_total",
			Help: "Stamped publishes attempted",
		}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "relay_edge_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
	}
	m.up.Set(1)

	uptime := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "relay_edge_uptime_seconds",
		Help: "Process uptime",
	}, func() float64 { return time.Since(started).Seconds() })

	reg.MustRegister(m.up, uptime, m.requests, m.errors, m.publishes, m.duration)
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

func (s *Server) withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.metrics == nil {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		s.metrics.requests.Inc()
		rw := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(rw, r)
		if rw.code >= 500 {
			s.metrics.errors.Inc()
		}
		// Label by the registered route pattern (e.g. "/v1/sites/{id}"), not
		// the raw path, so cardinality stays bounded to known routes rather
		// than growing with every guessed/unmatched URL.
		route := "unmatched"
		if _, pattern := s.Mux.Handler(r); pattern != "" {
			route = pattern
		}
		s.metrics.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
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

// Flush makes statusRecorder satisfy http.Flusher when the underlying
// ResponseWriter does. Without this, the embedded http.ResponseWriter
// field's static interface type (which has no Flush method) hides the
// concrete writer's Flusher support, so the SSE stream handlers'
// `w.(http.Flusher)` check fails and every /v1/*/stream endpoint 500s.
func (s *statusRecorder) Flush() {
	if fl, ok := s.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func (s *Server) IncPublish() {
	if s.metrics != nil {
		s.metrics.publishes.Inc()
	}
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if s.metrics == nil {
		w.WriteHeader(404)
		return
	}
	promhttp.HandlerFor(s.metrics.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}
