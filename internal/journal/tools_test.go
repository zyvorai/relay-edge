// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package journal

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestApplyPaths(t *testing.T) {
	orig := map[string]any{"a": map[string]any{"b": 1.0, "c": "x"}, "n": 5.0, "s": "str"}
	got, err := ApplyPaths(orig, map[string]any{"a.b": 2.0, "a.d.e": true, "z": "new"}, []string{"n", "a.c", "missing.deep", "s.x"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"a": map[string]any{"b": 2.0, "d": map[string]any{"e": true}}, "s": "str", "z": "new"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if orig["n"] != 5.0 || orig["a"].(map[string]any)["b"] != 1.0 {
		t.Fatal("input mutated")
	}
}

func TestApplyPathsRejects(t *testing.T) {
	d := map[string]any{"s": "str"}
	big := map[string]any{}
	for i := 0; i < MaxPathEdits+1; i++ {
		big[strings.Repeat("k", i+1)] = 1
	}
	cases := []struct {
		name string
		set  map[string]any
		rm   []string
	}{
		{"empty seg", map[string]any{"a..b": 1}, nil},
		{"empty path", map[string]any{"": 1}, nil},
		{"trailing dot", nil, []string{"a."}},
		{"too deep", map[string]any{"a.b.c.d.e.f.g.h.i": 1}, nil},
		{"through scalar", map[string]any{"s.x": 1}, nil},
		{"control", map[string]any{"a\nb": 1}, nil},
		{"long seg", map[string]any{strings.Repeat("x", 65): 1}, nil},
		{"too many", big, nil},
	}
	for _, c := range cases {
		if _, err := ApplyPaths(d, c.set, c.rm); err == nil {
			t.Errorf("%s: want error", c.name)
		}
	}
	if got, err := ApplyPaths(nil, nil, nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("nil data: %v %v", got, err)
	}
}

func TestGetAndMatchType(t *testing.T) {
	j := New(3)
	for i := 0; i < 5; i++ {
		j.Add(Entry{Type: "a.b", Data: map[string]any{"i": i}})
	}
	if _, ok := j.Get(1); ok {
		t.Fatal("rotated entry returned")
	}
	if _, ok := j.Get(99); ok {
		t.Fatal("future entry returned")
	}
	e, ok := j.Get(4)
	if !ok || e.Seq != 4 {
		t.Fatalf("%v %v", e, ok)
	}
	e.Data["i"] = "mutated"
	if e2, _ := j.Get(4); e2.Data["i"] == "mutated" {
		t.Fatal("Get must return a copy")
	}
	if !MatchType("fleet.x", []string{"fleet.*"}) || MatchType("fleetx", []string{"fleet.*"}) || !MatchType("a", []string{"a"}) {
		t.Fatal("MatchType")
	}
}

func TestSummarize(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	in := []Entry{
		{Seq: 1, At: t0, Type: "a.b", Severity: "info", Origin: "sim", Outcome: "published"},
		{Seq: 2, At: t0.Add(2 * time.Second), Type: "a.b", Severity: "critical", Origin: "sim", Outcome: "dropped"},
		{Seq: 3, At: t0.Add(4 * time.Second), Type: "c.d", Severity: "info"},
	}
	st := Summarize(in)
	if st.Count != 3 || st.FirstSeq != 1 || st.LastSeq != 3 || st.SpanSec != 4 || st.RatePerSec != 0.75 {
		t.Fatalf("%+v", st)
	}
	if st.ByType["a.b"] != 2 || st.BySeverity["info"] != 2 || st.ByOrigin["unknown"] != 1 || st.ByOutcome["pending"] != 1 || st.ByOutcome["dropped"] != 1 {
		t.Fatalf("%+v", st)
	}
	empty := Summarize(nil)
	if empty.Count != 0 || empty.FirstAt != nil || empty.ByType == nil {
		t.Fatalf("%+v", empty)
	}
}
