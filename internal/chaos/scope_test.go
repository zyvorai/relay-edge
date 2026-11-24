// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package chaos

import (
	"math"
	"testing"
)

func TestValidateCorrelationAndTypes(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"corr ok", Config{CorrelationPct: 100}, true},
		{"corr negative", Config{CorrelationPct: -1}, false},
		{"corr over", Config{CorrelationPct: 100.5}, false},
		{"corr nan", Config{CorrelationPct: math.NaN()}, false},
		{"types ok", Config{Types: []string{"fleet.*", "a.b"}}, true},
		{"types empty string", Config{Types: []string{""}}, false},
		{"types path", Config{Types: []string{"../x"}}, false},
		{"types query", Config{Types: []string{"a?x=1"}}, false},
		{"types bare star", Config{Types: []string{"*"}}, true},
		{"too many", Config{Types: make([]string, MaxTypes+1)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.cfg.Validate(); (err == nil) != c.ok {
				t.Fatalf("Validate()=%v want ok=%v", err, c.ok)
			}
		})
	}
}

// meanRun returns the mean length of consecutive drop runs over n decisions.
func meanRun(t *testing.T, cfg Config, n int) float64 {
	t.Helper()
	i := New()
	if err := i.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	runs, cur, total := 0, 0, 0
	for k := 0; k < n; k++ {
		if i.Decide().Action == Drop {
			cur++
			total++
			continue
		}
		if cur > 0 {
			runs++
			cur = 0
		}
	}
	if cur > 0 {
		runs++
	}
	if runs == 0 {
		return 0
	}
	return float64(total) / float64(runs)
}

func TestCorrelationMakesDropsBursty(t *testing.T) {
	base := Config{Enabled: true, Seed: 5, DropPct: 20}
	indep := meanRun(t, base, 20000)
	base.CorrelationPct = 80
	burst := meanRun(t, base, 20000)
	if burst < indep*1.8 {
		t.Fatalf("correlated mean run %.2f not clearly above independent %.2f", burst, indep)
	}
	// Deterministic for a seed.
	if meanRun(t, base, 2000) != meanRun(t, base, 2000) {
		t.Fatal("not deterministic")
	}
}

func TestCorrelationWithZeroDropNeverDrops(t *testing.T) {
	i := New()
	_ = i.Configure(Config{Enabled: true, Seed: 1, CorrelationPct: 100})
	for k := 0; k < 500; k++ {
		if i.Decide().Action != Pass {
			t.Fatal("dropped with drop_pct 0")
		}
	}
}

func TestTypesScopeSkipsWithoutConsumingRandomness(t *testing.T) {
	scoped := New()
	_ = scoped.Configure(Config{Enabled: true, Seed: 9, DropPct: 50, Types: []string{"fleet.*"}})
	plain := New()
	_ = plain.Configure(Config{Enabled: true, Seed: 9, DropPct: 50})
	for k := 0; k < 200; k++ {
		if d := scoped.DecideFor("other.thing"); d.Action != Pass {
			t.Fatal("out-of-scope event was faulted")
		}
		if got, want := scoped.DecideFor("fleet.robot.lost"), plain.Decide(); got != want {
			t.Fatalf("draw %d: scoped %v plain %v (out-of-scope events must not shift the sequence)", k, got, want)
		}
	}
	_, st := scoped.State()
	if st.Skipped != 200 || st.Seen != 200 {
		t.Fatalf("stats %+v", st)
	}
}
