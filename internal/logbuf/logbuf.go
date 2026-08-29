// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package logbuf

import (
	"io"
	"sync"
	"time"
)

// Ring is a thread-safe line buffer for recent process logs.
type Ring struct {
	mu    sync.RWMutex
	lines []string
	cap   int
}

func New(capacity int) *Ring {
	if capacity < 32 {
		capacity = 32
	}
	return &Ring{cap: capacity, lines: make([]string, 0, capacity)}
}

func (r *Ring) Write(p []byte) (int, error) {
	r.append(string(p))
	return len(p), nil
}

func (r *Ring) Append(s string) {
	r.append(s)
}

func (r *Ring) append(s string) {
	if s == "" {
		return
	}
	// trim trailing newline for storage; UI adds its own
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	if s == "" {
		return
	}
	line := time.Now().UTC().Format("15:04:05.000") + "Z " + s
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.lines) >= r.cap {
		copy(r.lines, r.lines[1:])
		r.lines[len(r.lines)-1] = line
		return
	}
	r.lines = append(r.lines, line)
}

func (r *Ring) Lines(n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n <= 0 || n > len(r.lines) {
		n = len(r.lines)
	}
	out := make([]string, n)
	copy(out, r.lines[len(r.lines)-n:])
	return out
}

// Multi returns a writer that tees to dst and the ring.
func Multi(dst io.Writer, r *Ring) io.Writer {
	return io.MultiWriter(dst, r)
}
