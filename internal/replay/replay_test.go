// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package replay

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zyvorai/relay-edge/internal/journal"
)

func stream(n int, gap time.Duration) []journal.Entry {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]journal.Entry, n)
	for i := range out {
		out[i] = journal.Entry{Type: "a.b", Severity: "info", Source: "s", At: t0.Add(time.Duration(i) * gap), Data: map[string]any{"i": float64(i)}}
	}
	return out
}

func TestOptionsNormalizeTable(t *testing.T) {
	cases := []struct {
		name string
		o    Options
		ok   bool
	}{
		{"defaults", Options{}, true},
		{"min", Options{Speed: MinSpeed}, true},
		{"max", Options{Speed: MaxSpeed}, true},
		{"too slow", Options{Speed: 0.001}, false},
		{"too fast", Options{Speed: 1001}, false},
		{"negative speed", Options{Speed: -1}, false},
		{"neg gap", Options{MaxGap: -time.Second}, false},
		{"huge gap", Options{MaxGap: time.Hour}, false},
		{"long run id", Options{RunID: strings.Repeat("a", 65)}, false},
		{"slash run id", Options{RunID: "a/b"}, false},
		{"space run id", Options{RunID: "a b"}, false},
		{"newline run id", Options{RunID: "a\nb"}, false},
		{"good run id", Options{RunID: "run-1_A.b"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.o.Normalize(); (err == nil) != c.ok {
				t.Fatalf("err=%v ok=%v", err, c.ok)
			}
		})
	}
}

func TestPlanDeterministicAndKeys(t *testing.T) {
	ev := stream(4, time.Second)
	a, oa, err := Plan(ev, Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, ob, _ := Plan(ev, Options{})
	if !reflect.DeepEqual(a, b) || oa.RunID != ob.RunID || !strings.HasPrefix(oa.RunID, "c") {
		t.Fatal("plan not deterministic")
	}
	if a[0].Key != "replay/"+oa.RunID+"/1" || a[3].Key != "replay/"+oa.RunID+"/4" {
		t.Fatalf("keys %q %q", a[0].Key, a[3].Key)
	}
	ev[2].Data["i"] = 99.0
	if _, o3, _ := Plan(ev, Options{}); o3.RunID == oa.RunID {
		t.Fatal("content change must change derived run id")
	}
	if _, o4, _ := Plan(ev, Options{RunID: "fixed"}); o4.RunID != "fixed" {
		t.Fatal("explicit run id ignored")
	}
}

func TestPlanSpeedAndGapCap(t *testing.T) {
	ev := stream(3, 10*time.Second)
	p, _, _ := Plan(ev, Options{Speed: 1, MaxGap: 30 * time.Second})
	if p[0].Delay != 0 || p[1].Delay != 10*time.Second {
		t.Fatalf("%v", p[1].Delay)
	}
	p, _, _ = Plan(ev, Options{Speed: 10})
	if p[1].Delay != time.Second {
		t.Fatalf("speed 10: %v", p[1].Delay)
	}
	p, _, _ = Plan(ev, Options{Speed: 1, MaxGap: 2 * time.Second})
	if p[1].Delay != 2*time.Second {
		t.Fatalf("cap: %v", p[1].Delay)
	}
}

func TestPlanOddTimestamps(t *testing.T) {
	ev := stream(3, time.Second)
	ev[1].At = time.Time{}
	ev[2].At = ev[0].At.Add(-time.Hour) // goes backwards
	p, _, err := Plan(ev, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p {
		if s.Delay != 0 {
			t.Fatalf("zero/backwards timestamps must give 0 delay: %+v", s)
		}
	}
}

func TestPlanLimits(t *testing.T) {
	if _, _, err := Plan(nil, Options{}); err == nil {
		t.Fatal("empty must fail")
	}
	if _, _, err := Plan(stream(MaxEvents+1, 0), Options{}); err == nil {
		t.Fatal("over MaxEvents must fail")
	}
	if _, _, err := Plan(stream(MaxEvents, 0), Options{}); err != nil {
		t.Fatal(err)
	}
}

func fastRunner() (*Runner, *[]time.Duration) {
	var mu sync.Mutex
	var slept []time.Duration
	r := NewRunner()
	r.Sleep = func(ctx context.Context, d time.Duration) error {
		mu.Lock()
		slept = append(slept, d)
		mu.Unlock()
		return ctx.Err()
	}
	return r, &slept
}

func TestRunnerPublishesInOrder(t *testing.T) {
	r, slept := fastRunner()
	var keys []string
	st, _, err := r.Start(stream(5, time.Second), Options{RunID: "r1", Speed: 2}, func(s Step) error {
		keys = append(keys, s.Key)
		return nil
	})
	if err != nil || st.State != "running" {
		t.Fatalf("%v %+v", err, st)
	}
	r.Wait()
	fin := r.Status()
	if fin.State != "done" || fin.Sent != 5 || fin.Failed != 0 || fin.FinishedAt == nil {
		t.Fatalf("%+v", fin)
	}
	want := []string{"replay/r1/1", "replay/r1/2", "replay/r1/3", "replay/r1/4", "replay/r1/5"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("%v", keys)
	}
	if (*slept)[1] != 500*time.Millisecond {
		t.Fatalf("slept %v", *slept)
	}
}

func TestRunnerCountsFailures(t *testing.T) {
	r, _ := fastRunner()
	_, _, _ = r.Start(stream(4, 0), Options{}, func(s Step) error {
		if s.Index%2 == 0 {
			return errors.New("boom")
		}
		return nil
	})
	r.Wait()
	if st := r.Status(); st.Sent != 2 || st.Failed != 2 || st.LastError != "boom" || st.State != "done" {
		t.Fatalf("%+v", st)
	}
}

func TestRunnerBusyAndCancel(t *testing.T) {
	r := NewRunner()
	block := make(chan struct{})
	r.Sleep = func(ctx context.Context, _ time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-block:
			return nil
		}
	}
	_, _, err := r.Start(stream(3, time.Second), Options{}, func(Step) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Start(stream(1, 0), Options{}, func(Step) error { return nil }); !errors.Is(err, ErrBusy) {
		t.Fatalf("err=%v", err)
	}
	if !r.Cancel() {
		t.Fatal("cancel reported nothing running")
	}
	r.Wait()
	if st := r.Status(); st.State != "cancelled" {
		t.Fatalf("%+v", st)
	}
	if r.Cancel() {
		t.Fatal("cancel on idle runner")
	}
	// a finished runner accepts a new run
	r.Sleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	if _, _, err := r.Start(stream(1, 0), Options{}, func(Step) error { return nil }); err != nil {
		t.Fatal(err)
	}
	r.Wait()
}

func TestRunnerDryRunPublishesNothing(t *testing.T) {
	r, _ := fastRunner()
	called := false
	st, steps, err := r.Start(stream(3, time.Second), Options{DryRun: true}, func(Step) error { called = true; return nil })
	if err != nil || called || st.State != "done" || !st.DryRun || len(steps) != 3 {
		t.Fatalf("%v called=%v %+v", err, called, st)
	}
}

func TestRunnerRejectsBadInput(t *testing.T) {
	r, _ := fastRunner()
	if _, _, err := r.Start(nil, Options{}, nil); err == nil {
		t.Fatal("empty")
	}
	if _, _, err := r.Start(stream(1, 0), Options{Speed: -2}, nil); err == nil {
		t.Fatal("speed")
	}
	if r.Status().State == "running" {
		t.Fatal("failed start must not leave runner busy")
	}
}

func TestRepeatPlanKeysAndLimits(t *testing.T) {
	steps, o, err := Plan(stream(3, time.Second), Options{Repeat: 2, RunID: "r"})
	if err != nil || o.Repeat != 2 || len(steps) != 6 {
		t.Fatalf("%v %d", err, len(steps))
	}
	want := []string{"replay/r/1", "replay/r/2", "replay/r/3", "replay/r/p2/1", "replay/r/p2/2", "replay/r/p2/3"}
	for i, s := range steps {
		if s.Key != want[i] || s.Index != i+1 {
			t.Fatalf("step %d: %+v", i, s)
		}
	}
	if steps[3].Delay != 0 {
		t.Fatal("a new pass must start without a pause")
	}
	for _, n := range []int{-1, MaxRepeat + 1} {
		if _, _, err := Plan(stream(1, 0), Options{Repeat: n}); err == nil {
			t.Fatalf("repeat %d accepted", n)
		}
	}
	if _, _, err := Plan(stream(MaxEvents, 0), Options{Repeat: 11}); err == nil {
		t.Fatal("total step cap not enforced")
	}
}

func TestSummarize(t *testing.T) {
	if Summarize(nil) != nil {
		t.Fatal("nil expected")
	}
	var d []time.Duration
	for i := 1; i <= 100; i++ {
		d = append(d, time.Duration(i)*time.Millisecond)
	}
	l := Summarize(d)
	if l.Min != 1 || l.P50 != 50 || l.P95 != 95 || l.Max != 100 {
		t.Fatalf("%+v", l)
	}
}

func TestRunRecordsLatencyAndRepeat(t *testing.T) {
	r := NewRunner()
	r.Sleep = func(context.Context, time.Duration) error { return nil }
	var n int
	var mu sync.Mutex
	st, _, err := r.Start(stream(2, time.Second), Options{Repeat: 3}, func(Step) error { mu.Lock(); n++; mu.Unlock(); return nil })
	if err != nil || st.Total != 6 || st.Repeat != 3 {
		t.Fatalf("%v %+v", err, st)
	}
	r.Wait()
	got := r.Status()
	if got.Sent != 6 || n != 6 || got.Latency == nil || got.Latency.Max < got.Latency.Min {
		t.Fatalf("%+v", got)
	}
}
