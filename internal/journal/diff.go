// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package journal

import (
	"encoding/json"
	"reflect"
	"strings"
)

// MaxDiffItems caps how many differences a Diff reports in detail.
const MaxDiffItems = 100

// Filter keeps events whose Type matches one of types (exact, or a prefix
// when the pattern ends in "*") and whose Severity is in severities. An empty
// list means "any". Order is preserved; the input is not modified.
func Filter(in []Entry, types, severities []string) []Entry {
	if len(types) == 0 && len(severities) == 0 {
		return in
	}
	out := make([]Entry, 0, len(in))
	for _, e := range in {
		if len(types) > 0 && !matchType(e.Type, types) {
			continue
		}
		if len(severities) > 0 && !contains(severities, e.Severity) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func matchType(t string, patterns []string) bool {
	for _, p := range patterns {
		if pre, ok := strings.CutSuffix(p, "*"); ok {
			if strings.HasPrefix(t, pre) {
				return true
			}
		} else if t == p {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// DiffOptions tunes what counts as a difference.
type DiffOptions struct {
	IncludeKey bool // also compare idempotency keys (off by default: keys differ between runs)
}

// DiffItem is one positional difference.
type DiffItem struct {
	Index    int      `json:"index"` // 1-based position
	Kind     string   `json:"kind"`  // changed | missing | unexpected
	Fields   []string `json:"fields,omitempty"`
	Expected *Entry   `json:"expected,omitempty"`
	Actual   *Entry   `json:"actual,omitempty"`
}

// DiffResult compares an expected stream with an actual one.
type DiffResult struct {
	Identical  bool `json:"identical"`
	Expected   int  `json:"expected"`
	Actual     int  `json:"actual"`
	Changed    int  `json:"changed"`
	Missing    int  `json:"missing"`    // in expected, absent from actual (tail)
	Unexpected int  `json:"unexpected"` // in actual, absent from expected (tail)
	// Reordered is true when no event differs in content but the order does:
	// the two streams hold the same events as multisets.
	Reordered bool       `json:"reordered"`
	Items     []DiffItem `json:"items"`
	Truncated bool       `json:"truncated"`
}

// Diff aligns the two streams by position and compares type, severity, source
// and data (plus the key if asked). Seq, time and outcome are ignored. It is
// pure and deterministic.
func Diff(expected, actual []Entry, o DiffOptions) DiffResult {
	r := DiffResult{Expected: len(expected), Actual: len(actual), Items: []DiffItem{}}
	add := func(it DiffItem) {
		if len(r.Items) < MaxDiffItems {
			r.Items = append(r.Items, it)
		} else {
			r.Truncated = true
		}
	}
	n := min(len(expected), len(actual))
	for i := 0; i < n; i++ {
		if f := diffFields(expected[i], actual[i], o); len(f) > 0 {
			r.Changed++
			e, a := expected[i], actual[i]
			add(DiffItem{Index: i + 1, Kind: "changed", Fields: f, Expected: &e, Actual: &a})
		}
	}
	for i := n; i < len(expected); i++ {
		r.Missing++
		e := expected[i]
		add(DiffItem{Index: i + 1, Kind: "missing", Expected: &e})
	}
	for i := n; i < len(actual); i++ {
		r.Unexpected++
		a := actual[i]
		add(DiffItem{Index: i + 1, Kind: "unexpected", Actual: &a})
	}
	r.Identical = r.Changed == 0 && r.Missing == 0 && r.Unexpected == 0
	if !r.Identical && r.Missing == 0 && r.Unexpected == 0 {
		r.Reordered = sameMultiset(expected, actual, o)
	}
	return r
}

func diffFields(a, b Entry, o DiffOptions) []string {
	var f []string
	if a.Type != b.Type {
		f = append(f, "type")
	}
	if a.Severity != b.Severity {
		f = append(f, "severity")
	}
	if a.Source != b.Source {
		f = append(f, "source")
	}
	if o.IncludeKey && a.Key != b.Key {
		f = append(f, "idempotency_key")
	}
	if !reflect.DeepEqual(a.Data, b.Data) {
		f = append(f, "data")
	}
	return f
}

func fingerprint(e Entry, o DiffOptions) string {
	k := ""
	if o.IncludeKey {
		k = e.Key
	}
	b, _ := json.Marshal([]any{e.Type, e.Severity, e.Source, k, e.Data}) // map keys marshal sorted
	return string(b)
}

func sameMultiset(a, b []Entry, o DiffOptions) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[string]int{}
	for _, e := range a {
		count[fingerprint(e, o)]++
	}
	for _, e := range b {
		k := fingerprint(e, o)
		if count[k] == 0 {
			return false
		}
		count[k]--
	}
	return true
}
