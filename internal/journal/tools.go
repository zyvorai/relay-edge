// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package journal

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Limits for ApplyPaths.
const (
	MaxPathDepth = 8
	MaxPathEdits = 64
)

// MatchType reports whether t matches one of patterns (exact, or a prefix when
// the pattern ends in "*").
func MatchType(t string, patterns []string) bool { return matchType(t, patterns) }

// Get returns a copy of the retained entry with the given sequence number.
func (j *Journal) Get(seq int64) (Entry, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := len(j.entries) - 1; i >= 0; i-- {
		if j.entries[i].Seq == seq {
			e := j.entries[i]
			e.Data = cloneMap(e.Data)
			return e, true
		}
		if j.entries[i].Seq < seq {
			break
		}
	}
	return Entry{}, false
}

// ApplyPaths returns a deep copy of data with dotted-path edits applied: set
// writes (creating intermediate objects), remove deletes. Paths address object
// keys only ("a.b.c"); array indexing is not supported. Removes run after sets.
func ApplyPaths(data map[string]any, set map[string]any, remove []string) (map[string]any, error) {
	if len(set)+len(remove) > MaxPathEdits {
		return nil, fmt.Errorf("at most %d edits", MaxPathEdits)
	}
	out := cloneMap(data)
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic when paths overlap
	for _, k := range keys {
		segs, err := splitPath(k)
		if err != nil {
			return nil, err
		}
		cur := out
		for _, s := range segs[:len(segs)-1] {
			next, ok := cur[s].(map[string]any)
			if !ok {
				if _, exists := cur[s]; exists {
					return nil, fmt.Errorf("path %q crosses a non-object value at %q", k, s)
				}
				next = map[string]any{}
				cur[s] = next
			}
			cur = next
		}
		cur[segs[len(segs)-1]] = set[k]
	}
	for _, k := range remove {
		segs, err := splitPath(k)
		if err != nil {
			return nil, err
		}
		cur := out
		for _, s := range segs[:len(segs)-1] {
			next, ok := cur[s].(map[string]any)
			if !ok {
				cur = nil
				break
			}
			cur = next
		}
		if cur != nil {
			delete(cur, segs[len(segs)-1])
		}
	}
	return out, nil
}

func splitPath(p string) ([]string, error) {
	segs := strings.Split(p, ".")
	if len(segs) > MaxPathDepth {
		return nil, fmt.Errorf("path %q deeper than %d", p, MaxPathDepth)
	}
	for _, s := range segs {
		if s == "" || len(s) > 64 || !noControl(s) {
			return nil, fmt.Errorf("invalid path %q", p)
		}
	}
	return segs, nil
}

// Stats summarizes a set of entries.
type Stats struct {
	Count      int            `json:"count"`
	FirstSeq   int64          `json:"first_seq,omitempty"`
	LastSeq    int64          `json:"last_seq,omitempty"`
	FirstAt    *time.Time     `json:"first_at,omitempty"`
	LastAt     *time.Time     `json:"last_at,omitempty"`
	SpanSec    float64        `json:"span_sec"`
	RatePerSec float64        `json:"rate_per_sec,omitempty"` // events per second over the span
	ByType     map[string]int `json:"by_type"`
	BySeverity map[string]int `json:"by_severity"`
	ByOrigin   map[string]int `json:"by_origin"`
	ByOutcome  map[string]int `json:"by_outcome"`
}

// Summarize counts entries by type, severity, origin and outcome. An empty
// origin or outcome is bucketed as "unknown" / "pending".
func Summarize(in []Entry) Stats {
	st := Stats{Count: len(in), ByType: map[string]int{}, BySeverity: map[string]int{}, ByOrigin: map[string]int{}, ByOutcome: map[string]int{}}
	if len(in) == 0 {
		return st
	}
	for _, e := range in {
		o, oc := e.Origin, e.Outcome
		if o == "" {
			o = "unknown"
		}
		if oc == "" {
			oc = "pending"
		}
		st.ByType[e.Type]++
		st.BySeverity[e.Severity]++
		st.ByOrigin[o]++
		st.ByOutcome[oc]++
	}
	first, last := in[0], in[len(in)-1]
	st.FirstSeq, st.LastSeq = first.Seq, last.Seq
	fa, la := first.At, last.At
	st.FirstAt, st.LastAt = &fa, &la
	if d := la.Sub(fa).Seconds(); d > 0 {
		st.SpanSec = d
		st.RatePerSec = math.Round(float64(len(in))/d*100) / 100
	}
	return st
}
