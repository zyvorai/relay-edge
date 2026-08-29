// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiToken == "" || isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		got := bearerToken(r)
		if got == "" {
			got = strings.TrimSpace(r.Header.Get("X-Edge-Token"))
		}
		if got == "" {
			got = strings.TrimSpace(r.URL.Query().Get("token"))
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.apiToken)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="relay-edge"`)
			writeJSON(w, 401, map[string]any{"error": "unauthorized", "hint": "set Authorization: Bearer <EDGE_API_TOKEN>"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isPublicPath(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/version", "/metrics":
		return true
	}
	return strings.HasPrefix(path, "/ui/") || path == "/ui"
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const p = "Bearer "
	if len(h) < len(p) || !strings.EqualFold(h[:len(p)], p) {
		return ""
	}
	return strings.TrimSpace(h[len(p):])
}
