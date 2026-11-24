// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

// Package chaos injects seeded, bounded faults (drop, duplicate, error, delay)
// into the publish path so Relay's idempotency, ordering and retry behaviour
// can be exercised without a flaky network. Decisions are a pure function of
// (seed, call order), so a failing run can be repeated exactly.
package chaos

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/zyvorai/relay-edge/internal/journal"
)

// Hard limits; Validate rejects anything beyond them.
const (
	MaxDelayMS  = 10_000
	MaxJitterMS = 10_000
)

// Config is the operator-facing fault profile. Percentages are 0..100.
type Config struct {
	Enabled      bool    `json:"enabled"`
	Seed         int64   `json:"seed"`
	DropPct      float64 `json:"drop_pct"`
	DuplicatePct float64 `json:"duplicate_pct"`
	ErrorPct     float64 `json:"error_pct"`
	DelayMS      int     `json:"delay_ms"`
	JitterMS     int     `json:"jitter_ms"`
	// CorrelationPct (0..100) makes drops bursty, like Toxiproxy's packet_loss
	// correlation: the drop probability is mixed with "the previous event was
	// dropped", so 0 is independent loss and 100 keeps dropping once started.
	CorrelationPct float64 `json:"correlation_pct"`
	// Types scopes the faults to matching event types (exact, or a trailing *
	// prefix). Other events pass untouched and consume no randomness. Empty
	// means every event.
	Types []string `json:"types,omitempty"`
}

// MaxTypes bounds the Types scope list.
const MaxTypes = 32

// Validate checks every field against its bound.
func (c Config) Validate() error {
	for name, p := range map[string]float64{"drop_pct": c.DropPct, "duplicate_pct": c.DuplicatePct, "error_pct": c.ErrorPct} {
		if p != p || p < 0 || p > 100 { // p != p catches NaN
			return fmt.Errorf("%s must be between 0 and 100", name)
		}
	}
	if c.CorrelationPct != c.CorrelationPct || c.CorrelationPct < 0 || c.CorrelationPct > 100 {
		return errors.New("correlation_pct must be between 0 and 100")
	}
	if len(c.Types) > MaxTypes {
		return fmt.Errorf("at most %d types", MaxTypes)
	}
	for _, t := range c.Types {
		base := strings.TrimSuffix(t, "*")
		if t == "" || (base != "" && !journal.ValidType(base)) || len(t) > 129 {
			return fmt.Errorf("invalid type pattern %q", t)
		}
	}
	if c.DelayMS < 0 || c.DelayMS > MaxDelayMS {
		return fmt.Errorf("delay_ms must be between 0 and %d", MaxDelayMS)
	}
	if c.JitterMS < 0 || c.JitterMS > MaxJitterMS {
		return fmt.Errorf("jitter_ms must be between 0 and %d", MaxJitterMS)
	}
	return nil
}

// Action is the fate chosen for one event.
type Action string

const (
	Pass      Action = "pass"
	Drop      Action = "drop"
	Duplicate Action = "duplicate"
	Error     Action = "error"
)

// ErrInjected is returned in place of a publish when Action is Error.
var ErrInjected = errors.New("chaos: injected publish error")

// Decision is the outcome for a single event.
type Decision struct {
	Action Action        `json:"action"`
	Delay  time.Duration `json:"delay_ns"`
}

// Stats counts decisions since the last Configure/Reset.
type Stats struct {
	Seen       int64 `json:"seen"`
	Passed     int64 `json:"passed"`
	Dropped    int64 `json:"dropped"`
	Duplicated int64 `json:"duplicated"`
	Errored    int64 `json:"errored"`
	Delayed    int64 `json:"delayed"`
	Skipped    int64 `json:"skipped"` // outside the Types scope
}

// Injector holds the current profile and a seeded source.
type Injector struct {
	mu       sync.Mutex
	cfg      Config
	rng      *rand.Rand
	stats    Stats
	lastDrop bool
}

// New returns a disabled injector.
func New() *Injector { return &Injector{rng: rand.New(rand.NewSource(0))} }

// Configure validates and installs cfg, reseeding the source and zeroing stats.
func (i *Injector) Configure(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.cfg = cfg
	i.rng = rand.New(rand.NewSource(cfg.Seed))
	i.stats = Stats{}
	i.lastDrop = false
	return nil
}

// Reset disables chaos and clears stats.
func (i *Injector) Reset() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.cfg = Config{}
	i.rng = rand.New(rand.NewSource(0))
	i.stats = Stats{}
	i.lastDrop = false
}

// State returns the current profile and counters.
func (i *Injector) State() (Config, Stats) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.cfg, i.stats
}

// Decide picks the fate of the next event. When disabled it always passes and
// consumes no randomness, so enabling chaos later does not shift a seeded run.
func (i *Injector) Decide() Decision { return i.DecideFor("") }

// DecideFor is Decide for an event of the given type. When the profile has a
// Types scope and evType is outside it, the event passes untouched (counted in
// Skipped) and no randomness is consumed.
func (i *Injector) DecideFor(evType string) Decision {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.cfg.Enabled {
		return Decision{Action: Pass}
	}
	if len(i.cfg.Types) > 0 && !journal.MatchType(evType, i.cfg.Types) {
		i.stats.Skipped++
		return Decision{Action: Pass}
	}
	i.stats.Seen++
	// Fixed draw order keeps decisions stable when only one knob changes.
	rDrop, rErr, rDup := i.rng.Float64()*100, i.rng.Float64()*100, i.rng.Float64()*100
	var delay time.Duration
	if i.cfg.DelayMS > 0 || i.cfg.JitterMS > 0 {
		ms := i.cfg.DelayMS
		if i.cfg.JitterMS > 0 {
			ms += i.rng.Intn(i.cfg.JitterMS + 1)
		}
		delay = time.Duration(ms) * time.Millisecond
	}
	dropP := i.cfg.DropPct
	if c := i.cfg.CorrelationPct / 100; c > 0 {
		prev := 0.0
		if i.lastDrop {
			prev = 100
		}
		dropP = dropP*(1-c) + prev*c
	}
	i.lastDrop = rDrop < dropP
	d := Decision{Action: Pass, Delay: delay}
	switch {
	case rDrop < dropP:
		d = Decision{Action: Drop}
		i.stats.Dropped++
		return d
	case rErr < i.cfg.ErrorPct:
		d = Decision{Action: Error}
		i.stats.Errored++
		return d
	case rDup < i.cfg.DuplicatePct:
		d.Action = Duplicate
		i.stats.Duplicated++
	default:
		i.stats.Passed++
	}
	if delay > 0 {
		i.stats.Delayed++
	}
	return d
}
