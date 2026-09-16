// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// tokenBucket is a simple, lazily-refilled per-client rate limiter. No
// background goroutine: refill is computed from elapsed time on each call.
type tokenBucket struct {
	tokens       float64
	max          float64
	refillPerSec float64
	last         time.Time
}

func (b *tokenBucket) allow(now time.Time) bool {
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.refillPerSec
		if b.tokens > b.max {
			b.tokens = b.max
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rateLimiter tracks one tokenBucket per client IP. Buckets untouched for
// staleAfter are opportunistically evicted on access so the map doesn't grow
// unbounded, without needing a background sweep goroutine.
type rateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*tokenBucket
	rps          float64
	burst        float64
	staleAfter   time.Duration
	now          func() time.Time
	lastEviction time.Time
}

func newRateLimiter(rps float64, burst int) *rateLimiter {
	if burst < 1 {
		burst = 1
	}
	return &rateLimiter{
		buckets:    map[string]*tokenBucket{},
		rps:        rps,
		burst:      float64(burst),
		staleAfter: 10 * time.Minute,
		now:        time.Now,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	now := rl.now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if now.Sub(rl.lastEviction) > rl.staleAfter {
		for k, b := range rl.buckets {
			if now.Sub(b.last) > rl.staleAfter {
				delete(rl.buckets, k)
			}
		}
		rl.lastEviction = now
	}

	b, ok := rl.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: rl.burst, max: rl.burst, refillPerSec: rl.rps, last: now}
		rl.buckets[key] = b
	}
	return b.allow(now)
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// withRateLimit throttles per-client-IP request rate when a limiter is
// configured (nil = disabled, the default, so upgrading an existing
// deployment doesn't silently change behavior). GETs to public read-only
// paths are exempt; writes are limited even on otherwise-public paths.
func (s *Server) withRateLimit(next http.Handler) http.Handler {
	if s.rateLimiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if !s.rateLimiter.allow(clientKey(r)) {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, 429, map[string]any{"error": "rate limited"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
