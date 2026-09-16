// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package logbuf

import (
	"bytes"
	"strings"
	"testing"
)

func TestRing_Wraps(t *testing.T) {
	const capacity = 32 // New()'s floor — capacities below this are bumped up to it
	r := New(capacity)
	for i := range capacity + 2 {
		r.Append("line" + string(rune('a'+i%26)))
	}

	got := r.Lines(0)
	if len(got) != capacity {
		t.Fatalf("len(Lines(0)) = %d, want %d", len(got), capacity)
	}
	// The oldest 2 of the cap+2 appended lines should have been evicted.
	if strings.HasSuffix(got[0], "linea") {
		t.Fatalf("oldest line %q should have been evicted", got[0])
	}
}

func TestRing_MinCapacity(t *testing.T) {
	r := New(1)
	if r.cap != 32 {
		t.Fatalf("cap = %d, want 32 (floor)", r.cap)
	}
}

func TestRing_TrimsTrailingNewlines(t *testing.T) {
	r := New(32)
	r.Append("hello\r\n")
	got := r.Lines(1)
	if len(got) != 1 {
		t.Fatalf("want 1 line, got %d", len(got))
	}
	if strings.HasSuffix(got[0], "\n") || strings.HasSuffix(got[0], "\r") {
		t.Fatalf("line still has trailing newline: %q", got[0])
	}
	if !strings.HasSuffix(got[0], "hello") {
		t.Fatalf("line = %q, want suffix %q", got[0], "hello")
	}
}

func TestRing_SkipsEmpty(t *testing.T) {
	r := New(32)
	r.Append("")
	r.Append("\n")
	r.Append("\r\n")
	if got := r.Lines(0); len(got) != 0 {
		t.Fatalf("want 0 lines for all-empty appends, got %v", got)
	}
}

func TestRing_WriteImplementsWriter(t *testing.T) {
	r := New(32)
	var buf bytes.Buffer
	w := Multi(&buf, r)

	n, err := w.Write([]byte("hello\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len("hello\n") {
		t.Fatalf("n = %d, want %d", n, len("hello\n"))
	}
	if buf.String() != "hello\n" {
		t.Fatalf("tee target = %q, want %q", buf.String(), "hello\n")
	}
	got := r.Lines(1)
	if len(got) != 1 || !strings.HasSuffix(got[0], "hello") {
		t.Fatalf("ring got %v, want a line ending in %q", got, "hello")
	}
}

func TestRing_LinesNClamped(t *testing.T) {
	r := New(32)
	r.Append("a")
	r.Append("b")

	if got := r.Lines(100); len(got) != 2 {
		t.Fatalf("Lines(100) = %d lines, want 2 (clamped to available)", len(got))
	}
	if got := r.Lines(-1); len(got) != 2 {
		t.Fatalf("Lines(-1) = %d lines, want 2 (all)", len(got))
	}
}
