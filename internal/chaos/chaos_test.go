// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package chaos

import (
	"math"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"zero", Config{}, true},
		{"full", Config{Enabled: true, DropPct: 100, DuplicatePct: 100, ErrorPct: 100, DelayMS: MaxDelayMS, JitterMS: MaxJitterMS}, true},
		{"negative drop", Config{DropPct: -0.1}, false},
		{"drop over", Config{DropPct: 100.1}, false},
		{"nan", Config{DuplicatePct: math.NaN()}, false},
		{"inf", Config{ErrorPct: math.Inf(1)}, false},
		{"delay over", Config{DelayMS: MaxDelayMS + 1}, false},
		{"negative delay", Config{DelayMS: -1}, false},
		{"jitter over", Config{JitterMS: MaxJitterMS + 1}, false},
		{"huge int", Config{DelayMS: math.MaxInt32}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.cfg.Validate(); (err == nil) != c.ok {
				t.Fatalf("Validate()=%v, want ok=%v", err, c.ok)
			}
		})
	}
}

func TestConfigureRejectsInvalidAndKeepsState(t *testing.T) {
	i := New()
	if err := i.Configure(Config{Enabled: true, DropPct: 100}); err != nil {
		t.Fatal(err)
	}
	if err := i.Configure(Config{Enabled: true, DropPct: 500}); err == nil {
		t.Fatal("expected error")
	}
	cfg, _ := i.State()
	if cfg.DropPct != 100 {
		t.Fatalf("invalid config replaced state: %+v", cfg)
	}
}

func run(t *testing.T, cfg Config, n int) []Decision {
	t.Helper()
	i := New()
	if err := i.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	out := make([]Decision, n)
	for k := range out {
		out[k] = i.Decide()
	}
	return out
}

func TestDecideDeterministicPerSeed(t *testing.T) {
	cfg := Config{Enabled: true, Seed: 99, DropPct: 20, DuplicatePct: 20, ErrorPct: 10, DelayMS: 5, JitterMS: 50}
	a, b := run(t, cfg, 500), run(t, cfg, 500)
	for k := range a {
		if a[k] != b[k] {
			t.Fatalf("decision %d differs: %+v vs %+v", k, a[k], b[k])
		}
	}
	cfg.Seed = 100
	c := run(t, cfg, 500)
	same := true
	for k := range a {
		if a[k] != c[k] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced identical sequences")
	}
}

func TestDecideExtremes(t *testing.T) {
	for _, d := range run(t, Config{Enabled: true, DropPct: 100}, 50) {
		if d.Action != Drop {
			t.Fatalf("want drop, got %v", d.Action)
		}
	}
	for _, d := range run(t, Config{Enabled: true, ErrorPct: 100}, 50) {
		if d.Action != Error {
			t.Fatalf("want error, got %v", d.Action)
		}
	}
	for _, d := range run(t, Config{Enabled: true, DuplicatePct: 100}, 50) {
		if d.Action != Duplicate {
			t.Fatalf("want duplicate, got %v", d.Action)
		}
	}
	for _, d := range run(t, Config{Enabled: true}, 50) {
		if d.Action != Pass || d.Delay != 0 {
			t.Fatalf("want clean pass, got %+v", d)
		}
	}
}

func TestDelayBounds(t *testing.T) {
	for _, d := range run(t, Config{Enabled: true, Seed: 3, DelayMS: 100, JitterMS: 40}, 300) {
		ms := d.Delay.Milliseconds()
		if ms < 100 || ms > 140 {
			t.Fatalf("delay %dms outside [100,140]", ms)
		}
	}
}

func TestDisabledPassesAndConsumesNothing(t *testing.T) {
	i := New()
	_ = i.Configure(Config{Enabled: false, Seed: 5, DropPct: 100})
	for k := 0; k < 10; k++ {
		if d := i.Decide(); d.Action != Pass {
			t.Fatalf("disabled chaos acted: %+v", d)
		}
	}
	_, st := i.State()
	if st.Seen != 0 {
		t.Fatalf("disabled chaos counted %d", st.Seen)
	}
}

func TestStatsAndReset(t *testing.T) {
	i := New()
	_ = i.Configure(Config{Enabled: true, Seed: 1, DropPct: 50, DelayMS: 1})
	for k := 0; k < 200; k++ {
		i.Decide()
	}
	_, st := i.State()
	if st.Seen != 200 || st.Dropped+st.Passed != 200 || st.Dropped == 0 || st.Passed == 0 {
		t.Fatalf("stats %+v", st)
	}
	i.Reset()
	cfg, st := i.State()
	if cfg.Enabled || st.Seen != 0 {
		t.Fatalf("reset left %+v %+v", cfg, st)
	}
}
