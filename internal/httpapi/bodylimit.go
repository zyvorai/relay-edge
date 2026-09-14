// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi

import "net/http"

// defaultMaxBodyBytes matches Helm production ingress proxy-body-size (8m).
const defaultMaxBodyBytes int64 = 8 << 20

// withBodyLimit caps request bodies for mutating methods so unbounded POSTs
// cannot exhaust memory. Oversize reads surface as HTTP 413 via MaxBytesReader.
func (s *Server) withBodyLimit(next http.Handler) http.Handler {
	max := s.maxBody
	if max <= 0 {
		max = defaultMaxBodyBytes
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			r.Body = http.MaxBytesReader(w, r.Body, max)
		}
		next.ServeHTTP(w, r)
	})
}
