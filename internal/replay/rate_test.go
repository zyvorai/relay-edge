// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package replay

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func near(a, b time.Duration) bool { return math.Abs(float64(a-b)) < float64(time.Millisecond) }

func TestPlanConstantRateIgnoresRecordedGaps(t *testing.T) {
	steps, o, err := Plan(stream(4, time.Hour), Options{RatePerSec: 10})
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Delay != 0 {
		t.Fatalf("first delay %v", steps[0].Delay)
	}
	for _, s := range steps[1:] {
		if !near(s.Delay, 100*time.Millisecond) {
			t.Fatalf("delay %v", s.Delay)
		}
	}
	if !o.rateMode() {
		t.Fatal("rate mode")
	}
}

func TestPlanRateStagesRamp(t *testing.T) {
	// 1 -> 11 events/s over 10s: gaps shrink from 1s towards 1/11s.
	steps, _, err := Plan(stream(6, 0), Options{StartRate: 1, Stages: []Stage{{DurationMS: 10000, RatePerSec: 11}}, MaxGap: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	prev := time.Hour
	for _, s := range steps[1:] {
		if s.Delay >= prev {
			t.Fatalf("ramp up must shorten gaps: %v then %v", prev, s.Delay)
		}
		prev = s.Delay
	}
	if !near(steps[1].Delay, time.Second) {
		t.Fatalf("first gap %v, want 1s at start rate", steps[1].Delay)
	}
}

func TestPlanStagesHoldLastRateAndAcrossRepeat(t *testing.T) {
	st := []Stage{{DurationMS: 100, RatePerSec: 20}, {DurationMS: 100, RatePerSec: 20}}
	steps, _, err := Plan(stream(10, 0), Options{Stages: st, Repeat: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 30 || !near(steps[29].Delay, 50*time.Millisecond) {
		t.Fatalf("len %d last %v", len(steps), steps[29].Delay)
	}
}

func TestPlanRateGapCapped(t *testing.T) {
	steps, _, _ := Plan(stream(3, 0), Options{RatePerSec: MinRate, MaxGap: time.Second})
	if steps[1].Delay != time.Second {
		t.Fatalf("%v", steps[1].Delay)
	}
}

func TestRateOptionsValidation(t *testing.T) {
	ok := []Stage{{DurationMS: 1000, RatePerSec: 5}}
	bad := []struct {
		name string
		o    Options
	}{
		{"rate and speed", Options{RatePerSec: 5, Speed: 2}},
		{"stages and speed", Options{Stages: ok, Speed: 1}},
		{"rate and stages", Options{RatePerSec: 5, Stages: ok}},
		{"rate too low", Options{RatePerSec: 0.001}},
		{"rate too high", Options{RatePerSec: 1001}},
		{"rate nan", Options{RatePerSec: math.NaN()}},
		{"start rate bad", Options{StartRate: 5000, Stages: ok}},
		{"zero duration", Options{Stages: []Stage{{DurationMS: 0, RatePerSec: 5}}}},
		{"huge duration", Options{Stages: []Stage{{DurationMS: MaxStageMS + 1, RatePerSec: 5}}}},
		{"stage rate zero", Options{Stages: []Stage{{DurationMS: 5, RatePerSec: 0}}}},
		{"too many stages", Options{Stages: make([]Stage, MaxStages+1)}},
		{"shuffle 1", Options{ShuffleWindow: 1}},
		{"shuffle negative", Options{ShuffleWindow: -2}},
		{"shuffle huge", Options{ShuffleWindow: MaxShuffleWin + 1}},
	}
	for _, c := range bad {
		if _, err := c.o.Normalize(); err == nil {
			t.Errorf("%s: want error", c.name)
		}
	}
	for _, o := range []Options{{RatePerSec: MaxRate}, {Stages: ok}, {ShuffleWindow: 2}} {
		if _, err := o.Normalize(); err != nil {
			t.Errorf("%+v: %v", o, err)
		}
	}
}

func TestShuffleWindowDeterministicKeepsKeysAndPacing(t *testing.T) {
	ev := stream(10, time.Second)
	plain, _, _ := Plan(ev, Options{RunID: "r"})
	a, _, err := Plan(ev, Options{RunID: "r", ShuffleWindow: 4, ShuffleSeed: 3})
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := Plan(ev, Options{RunID: "r", ShuffleWindow: 4, ShuffleSeed: 3})
	c, _, _ := Plan(ev, Options{RunID: "r", ShuffleWindow: 4, ShuffleSeed: 4})
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed must give the same order")
	}
	if reflect.DeepEqual(a, c) {
		t.Fatal("different seed should change the order")
	}
	moved := false
	for p, s := range a {
		if s.Delay != plain[p].Delay {
			t.Fatalf("pacing changed at %d", p)
		}
		// Key follows the event's original position.
		if want := "replay/r/" + itoa(int(s.Entry.Data["i"].(float64))+1); s.Key != want {
			t.Fatalf("slot %d key %q want %q", p, s.Key, want)
		}
		// Events never leave their window.
		if orig := int(s.Entry.Data["i"].(float64)); orig/4 != p/4 {
			t.Fatalf("event %d escaped its window to slot %d", orig, p)
		}
		if s.Entry.Data["i"] != plain[p].Entry.Data["i"] {
			moved = true
		}
	}
	if !moved {
		t.Fatal("nothing moved")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
