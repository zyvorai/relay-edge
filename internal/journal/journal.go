// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

// Package journal keeps a bounded in-memory record of stamped events as they
// were handed to the publish path, and converts them to and from NDJSON and
// CloudEvents 1.0 JSON. It is a diagnostic ring, not a durable event log:
// Relay owns durability.
package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Limits.
const (
	DefaultCap       = 5000
	MaxCap           = 100_000
	MaxEventBytes    = 64 << 10 // per imported event
	MaxSourceLen     = 256
	MaxKeyLen        = 256
	cloudEventsSpec  = "1.0"
	sourcePrefix     = "relay-edge/"
	MaxListPageLimit = 1000
)

var (
	typeRE     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	severityRE = regexp.MustCompile(`^[a-z]{1,16}$`)
)

// Entry is one journaled event, exactly as handed to the publish path.
type Entry struct {
	Seq      int64          `json:"seq"`
	At       time.Time      `json:"at"`
	Type     string         `json:"type"`
	Severity string         `json:"severity"`
	Source   string         `json:"source"`
	Key      string         `json:"idempotency_key"`
	Data     map[string]any `json:"data"`
	Origin   string         `json:"origin,omitempty"`  // sim | replay
	Outcome  string         `json:"outcome,omitempty"` // published | dropped | duplicated | error | dry_run
}

// ValidType reports whether t is a safe event type (it becomes a gateway URL
// path segment, so path characters and whitespace are rejected).
func ValidType(t string) bool { return typeRE.MatchString(t) }

// ValidSeverity reports whether s looks like a severity word.
func ValidSeverity(s string) bool { return severityRE.MatchString(s) }

func noControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// Journal is a fixed-capacity ring.
type Journal struct {
	mu      sync.Mutex
	cap     int
	next    int64
	entries []Entry
	now     func() time.Time
}

// New returns a journal holding at most capacity entries (clamped to 1..MaxCap;
// <=0 means DefaultCap).
func New(capacity int) *Journal {
	return &Journal{cap: clampCap(capacity), next: 1, now: func() time.Time { return time.Now().UTC() }}
}

func clampCap(c int) int {
	if c <= 0 {
		return DefaultCap
	}
	if c > MaxCap {
		return MaxCap
	}
	return c
}

// Add stores e (assigning Seq and, if zero, At) and returns the stored copy.
func (j *Journal) Add(e Entry) Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	e.Seq = j.next
	j.next++
	if e.At.IsZero() {
		e.At = j.now()
	}
	e.Data = cloneMap(e.Data)
	j.entries = append(j.entries, e)
	if over := len(j.entries) - j.cap; over > 0 {
		j.entries = append([]Entry(nil), j.entries[over:]...)
	}
	return e
}

// SetOutcome records the final fate of entry seq (no-op if it has rotated out).
func (j *Journal) SetOutcome(seq int64, outcome string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := len(j.entries) - 1; i >= 0; i-- {
		if j.entries[i].Seq == seq {
			j.entries[i].Outcome = outcome
			return
		}
		if j.entries[i].Seq < seq {
			return
		}
	}
}

// List returns entries with Seq > afterSeq, oldest first, at most limit
// (clamped to 1..MaxListPageLimit; <=0 means 100).
func (j *Journal) List(afterSeq int64, limit int) []Entry {
	if limit <= 0 {
		limit = 100
	}
	if limit > MaxListPageLimit {
		limit = MaxListPageLimit
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Entry, 0, limit)
	for _, e := range j.entries {
		if e.Seq > afterSeq {
			out = append(out, e)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

// Range returns entries with from <= Seq <= to (to<=0 means no upper bound),
// oldest first, at most max.
func (j *Journal) Range(from, to int64, max int) []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	var out []Entry
	for _, e := range j.entries {
		if e.Seq < from || (to > 0 && e.Seq > to) {
			continue
		}
		out = append(out, e)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

// Len is the number of retained entries.
func (j *Journal) Len() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.entries)
}

// Cap is the ring capacity.
func (j *Journal) Cap() int { return j.cap }

// Clear drops all entries; sequence numbers keep increasing.
func (j *Journal) Clear() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	n := len(j.entries)
	j.entries = nil
	return n
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return map[string]any{"_unserializable": true}
	}
	var o map[string]any
	if json.Unmarshal(b, &o) != nil || o == nil {
		return map[string]any{}
	}
	return o
}

// WriteNDJSON writes one JSON object per line.
func WriteNDJSON(w io.Writer, entries []Entry) error {
	enc := json.NewEncoder(w)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// CloudEvent is a CloudEvents 1.0 structured-mode JSON event.
type CloudEvent struct {
	SpecVersion     string         `json:"specversion"`
	ID              string         `json:"id"`
	Source          string         `json:"source"`
	Type            string         `json:"type"`
	Time            string         `json:"time,omitempty"`
	DataContentType string         `json:"datacontenttype,omitempty"`
	Data            map[string]any `json:"data"`
	Severity        string         `json:"severity,omitempty"`
	Sequence        int64          `json:"sequence,omitempty"`
}

// ToCloudEvents maps entries to CloudEvents 1.0 (batch media type
// application/cloudevents-batch+json). The idempotency key becomes the id,
// which is unique per source as the spec requires.
func ToCloudEvents(entries []Entry) []CloudEvent {
	out := make([]CloudEvent, 0, len(entries))
	for _, e := range entries {
		id := e.Key
		if id == "" {
			id = "seq-" + strconv.FormatInt(e.Seq, 10)
		}
		out = append(out, CloudEvent{
			SpecVersion:     cloudEventsSpec,
			ID:              id,
			Source:          sourcePrefix + url.PathEscape(e.Source),
			Type:            e.Type,
			Time:            e.At.UTC().Format(time.RFC3339Nano),
			DataContentType: "application/json",
			Data:            e.Data,
			Severity:        e.Severity,
			Sequence:        e.Seq,
		})
	}
	return out
}

type wireEvent struct {
	SpecVersion string            `json:"specversion"`
	ID          string            `json:"id"`
	Time        string            `json:"time"`
	At          string            `json:"at"`
	Type        string            `json:"type"`
	Severity    string            `json:"severity"`
	Source      string            `json:"source"`
	Key         string            `json:"idempotency_key"`
	Data        any               `json:"data"`
	Seq         int64             `json:"seq"`
	Sequence    int64             `json:"sequence"`
	Events      []json.RawMessage `json:"events"`
}

// ErrTooMany is returned when more than the allowed events are supplied.
var ErrTooMany = errors.New("journal: too many events")

// Parse reads events from raw, which may be a JSON array, an object with an
// "events" array, a single event, or NDJSON; each event may be a native Entry
// or a CloudEvents 1.0 object. It validates every event (type, severity,
// source, key, size, data must be an object) and returns at most max events,
// failing with ErrTooMany beyond that. Parsing is pure and deterministic.
func Parse(raw []byte, max int) ([]Entry, error) {
	if max <= 0 {
		return nil, fmt.Errorf("journal: max must be positive")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var out []Entry
	add := func(item json.RawMessage) error {
		if len(out) >= max {
			return ErrTooMany
		}
		e, err := parseOne(item)
		if err != nil {
			return fmt.Errorf("event %d: %w", len(out)+1, err)
		}
		out = append(out, e)
		return nil
	}
	for {
		var v json.RawMessage
		if err := dec.Decode(&v); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("journal: invalid JSON: %w", err)
		}
		t := bytes.TrimSpace(v)
		switch {
		case len(t) > 0 && t[0] == '[':
			var items []json.RawMessage
			if err := json.Unmarshal(t, &items); err != nil {
				return nil, fmt.Errorf("journal: invalid array: %w", err)
			}
			for _, it := range items {
				if err := add(it); err != nil {
					return nil, err
				}
			}
		case len(t) > 0 && t[0] == '{':
			var probe struct {
				Events []json.RawMessage `json:"events"`
				Type   string            `json:"type"`
			}
			if err := json.Unmarshal(t, &probe); err != nil {
				return nil, fmt.Errorf("journal: invalid object: %w", err)
			}
			if probe.Type == "" && probe.Events != nil {
				for _, it := range probe.Events {
					if err := add(it); err != nil {
						return nil, err
					}
				}
			} else if err := add(t); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("journal: events must be JSON objects or arrays")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("journal: no events")
	}
	return out, nil
}

func parseOne(item json.RawMessage) (Entry, error) {
	if len(item) > MaxEventBytes {
		return Entry{}, fmt.Errorf("event larger than %d bytes", MaxEventBytes)
	}
	var w wireEvent
	if err := json.Unmarshal(item, &w); err != nil {
		return Entry{}, err
	}
	e := Entry{Type: w.Type, Severity: w.Severity, Source: w.Source, Key: w.Key, Seq: w.Seq}
	ts := w.At
	if w.SpecVersion != "" { // CloudEvents
		if w.SpecVersion != cloudEventsSpec {
			return Entry{}, fmt.Errorf("unsupported specversion %q", w.SpecVersion)
		}
		if w.ID == "" || w.Source == "" || w.Type == "" {
			return Entry{}, errors.New("cloudevent requires id, source and type")
		}
		e.Key = w.ID
		e.Seq = w.Sequence
		ts = w.Time
		src := strings.TrimPrefix(w.Source, sourcePrefix)
		if u, err := url.PathUnescape(src); err == nil {
			src = u
		}
		e.Source = src
	}
	if !ValidType(e.Type) {
		return Entry{}, fmt.Errorf("invalid type %q", clip(e.Type))
	}
	if e.Severity == "" {
		e.Severity = "info"
	}
	if !ValidSeverity(e.Severity) {
		return Entry{}, fmt.Errorf("invalid severity %q", clip(e.Severity))
	}
	if len(e.Source) > MaxSourceLen || !noControl(e.Source) {
		return Entry{}, errors.New("invalid source")
	}
	if len(e.Key) > MaxKeyLen || !noControl(e.Key) {
		return Entry{}, errors.New("invalid idempotency_key")
	}
	switch d := w.Data.(type) {
	case nil:
		e.Data = map[string]any{}
	case map[string]any:
		e.Data = d
	default:
		return Entry{}, errors.New("data must be a JSON object")
	}
	if ts != "" {
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return Entry{}, fmt.Errorf("invalid timestamp: %w", err)
		}
		e.At = t.UTC()
	}
	return e, nil
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}
