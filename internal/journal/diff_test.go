// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package journal

import "testing"

func ent(typ, sev string, data map[string]any) Entry {
	return Entry{Type: typ, Severity: sev, Source: "s", Key: "k-" + typ, Data: data}
}

func TestFilter(t *testing.T) {
	in := []Entry{ent("fleet.a", "info", nil), ent("fleet.b", "critical", nil), ent("farm.c", "info", nil)}
	cases := []struct {
		name       string
		types, sev []string
		want       int
	}{
		{"none", nil, nil, 3},
		{"exact", []string{"farm.c"}, nil, 1},
		{"prefix", []string{"fleet.*"}, nil, 2},
		{"severity", nil, []string{"info"}, 2},
		{"both", []string{"fleet.*"}, []string{"info"}, 1},
		{"no match", []string{"nope"}, nil, 0},
		{"prefix is not substring", []string{"leet.*"}, nil, 0},
	}
	for _, c := range cases {
		if got := Filter(in, c.types, c.sev); len(got) != c.want {
			t.Errorf("%s: got %d want %d", c.name, len(got), c.want)
		}
	}
	if got := Filter(in, []string{"fleet.*"}, nil); got[0].Type != "fleet.a" || got[1].Type != "fleet.b" {
		t.Fatal("order must be preserved")
	}
}

func TestDiffIdenticalIgnoresSeqTimeKey(t *testing.T) {
	a := []Entry{ent("x.y", "info", map[string]any{"n": 1.0})}
	b := []Entry{ent("x.y", "info", map[string]any{"n": 1.0})}
	b[0].Seq, b[0].Key, b[0].Outcome = 99, "other", "published"
	if r := Diff(a, b, DiffOptions{}); !r.Identical || len(r.Items) != 0 || r.Reordered {
		t.Fatalf("%+v", r)
	}
	if r := Diff(a, b, DiffOptions{IncludeKey: true}); r.Identical || r.Items[0].Fields[0] != "idempotency_key" {
		t.Fatalf("%+v", r)
	}
}

func TestDiffChangedMissingUnexpectedReordered(t *testing.T) {
	x, y := ent("x", "info", map[string]any{"v": 1.0}), ent("y", "info", map[string]any{"v": 2.0})
	r := Diff([]Entry{x, y}, []Entry{x, ent("y", "critical", map[string]any{"v": 3.0})}, DiffOptions{})
	if r.Changed != 1 || len(r.Items[0].Fields) != 2 || r.Items[0].Index != 2 || r.Reordered {
		t.Fatalf("%+v", r)
	}
	r = Diff([]Entry{x, y}, []Entry{x}, DiffOptions{})
	if r.Missing != 1 || r.Items[0].Kind != "missing" || r.Identical {
		t.Fatalf("%+v", r)
	}
	r = Diff([]Entry{x}, []Entry{x, y}, DiffOptions{})
	if r.Unexpected != 1 || r.Items[0].Kind != "unexpected" {
		t.Fatalf("%+v", r)
	}
	r = Diff([]Entry{x, y}, []Entry{y, x}, DiffOptions{})
	if r.Identical || !r.Reordered || r.Changed != 2 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffTruncates(t *testing.T) {
	var a []Entry
	for i := 0; i < MaxDiffItems+5; i++ {
		a = append(a, ent("x", "info", nil))
	}
	r := Diff(a, nil, DiffOptions{})
	if r.Missing != len(a) || len(r.Items) != MaxDiffItems || !r.Truncated {
		t.Fatalf("%d %d %v", r.Missing, len(r.Items), r.Truncated)
	}
}
