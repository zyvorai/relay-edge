// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package journal

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRingCapAndSeq(t *testing.T) {
	j := New(3)
	for i := 0; i < 7; i++ {
		j.Add(Entry{Type: "a.b", Data: map[string]any{"i": i}})
	}
	if j.Len() != 3 {
		t.Fatalf("len=%d", j.Len())
	}
	got := j.List(0, 100)
	if got[0].Seq != 5 || got[2].Seq != 7 {
		t.Fatalf("seqs %d..%d", got[0].Seq, got[2].Seq)
	}
	if n := j.Clear(); n != 3 || j.Len() != 0 {
		t.Fatal("clear")
	}
	if e := j.Add(Entry{Type: "x"}); e.Seq != 8 {
		t.Fatalf("seq must keep increasing, got %d", e.Seq)
	}
}

func TestCapClamp(t *testing.T) {
	if New(0).Cap() != DefaultCap || New(-5).Cap() != DefaultCap || New(MaxCap*10).Cap() != MaxCap {
		t.Fatal("cap clamp")
	}
}

func TestAddClonesData(t *testing.T) {
	j := New(5)
	m := map[string]any{"k": "v"}
	e := j.Add(Entry{Type: "t", Data: m})
	m["k"] = "mutated"
	if j.List(0, 1)[0].Data["k"] != "v" || e.Data["k"] != "v" {
		t.Fatal("journal must not alias caller data")
	}
}

func TestSetOutcomeAndRange(t *testing.T) {
	j := New(10)
	for i := 0; i < 5; i++ {
		j.Add(Entry{Type: "t"})
	}
	j.SetOutcome(3, "dropped")
	j.SetOutcome(999, "ignored")
	r := j.Range(2, 4, 0)
	if len(r) != 3 || r[1].Outcome != "dropped" {
		t.Fatalf("%+v", r)
	}
	if len(j.Range(1, 0, 2)) != 2 {
		t.Fatal("max")
	}
}

func TestListLimitClamp(t *testing.T) {
	j := New(MaxListPageLimit + 50)
	for i := 0; i < MaxListPageLimit+20; i++ {
		j.Add(Entry{Type: "t"})
	}
	if n := len(j.List(0, 1_000_000)); n != MaxListPageLimit {
		t.Fatalf("n=%d", n)
	}
}

func TestParseTable(t *testing.T) {
	big := `{"type":"a.b","data":{"x":"` + strings.Repeat("A", MaxEventBytes) + `"}}`
	cases := []struct {
		name    string
		in      string
		max     int
		wantN   int
		wantErr string
	}{
		{"single", `{"type":"fleet.power.island","severity":"critical","data":{"a":1}}`, 10, 1, ""},
		{"array", `[{"type":"a"},{"type":"b"}]`, 10, 2, ""},
		{"events wrapper", `{"events":[{"type":"a"},{"type":"b"}]}`, 10, 2, ""},
		{"ndjson", "{\"type\":\"a\"}\n{\"type\":\"b\"}\n{\"type\":\"c\"}\n", 10, 3, ""},
		{"cloudevent", `{"specversion":"1.0","id":"k1","source":"relay-edge/Site%20A","type":"a.b","time":"2026-01-02T03:04:05Z","data":{"z":1}}`, 10, 1, ""},
		{"default severity", `{"type":"a"}`, 10, 1, ""},
		{"too many", `[{"type":"a"},{"type":"b"},{"type":"c"}]`, 2, 0, "too many"},
		{"empty array", `[]`, 10, 0, "no events"},
		{"empty input", ``, 10, 0, "no events"},
		{"bad json", `{"type":`, 10, 0, "invalid JSON"},
		{"scalar", `42`, 10, 0, "objects or arrays"},
		{"path type", `{"type":"../../admin"}`, 10, 0, "invalid type"},
		{"slash type", `{"type":"a/b"}`, 10, 0, "invalid type"},
		{"colon type", `{"type":"a:publish"}`, 10, 0, "invalid type"},
		{"space type", `{"type":"a b"}`, 10, 0, "invalid type"},
		{"missing type", `{"severity":"info"}`, 10, 0, "invalid type"},
		{"long type", `{"type":"` + strings.Repeat("a", 129) + `"}`, 10, 0, "invalid type"},
		{"bad severity", `{"type":"a","severity":"CRITICAL!"}`, 10, 0, "invalid severity"},
		{"ctrl in key", "{\"type\":\"a\",\"idempotency_key\":\"k\\u0000x\"}", 10, 0, "idempotency_key"},
		{"long key", `{"type":"a","idempotency_key":"` + strings.Repeat("k", MaxKeyLen+1) + `"}`, 10, 0, "idempotency_key"},
		{"ctrl in source", "{\"type\":\"a\",\"source\":\"s\\nx\"}", 10, 0, "source"},
		{"data array", `{"type":"a","data":[1,2]}`, 10, 0, "data must be"},
		{"data string", `{"type":"a","data":"x"}`, 10, 0, "data must be"},
		{"bad time", `{"type":"a","at":"yesterday"}`, 10, 0, "timestamp"},
		{"oversize event", big, 10, 0, "larger than"},
		{"ce wrong version", `{"specversion":"0.3","id":"1","source":"s","type":"a"}`, 10, 0, "specversion"},
		{"ce missing id", `{"specversion":"1.0","source":"s","type":"a"}`, 10, 0, "requires id"},
		{"error names position", `[{"type":"ok"},{"type":"bad/one"}]`, 10, 0, "event 2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse([]byte(c.in), c.max)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err=%v want substring %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != c.wantN {
				t.Fatalf("n=%d want %d", len(got), c.wantN)
			}
			if got[0].Severity == "" || got[0].Data == nil {
				t.Fatalf("defaults missing: %+v", got[0])
			}
		})
	}
}

func TestParseTooManySentinel(t *testing.T) {
	_, err := Parse([]byte(`[{"type":"a"},{"type":"b"}]`), 1)
	if !errors.Is(err, ErrTooMany) {
		t.Fatalf("err=%v", err)
	}
	if _, err := Parse([]byte(`{"type":"a"}`), 0); err == nil {
		t.Fatal("max=0 must fail")
	}
}

func TestParseDeepNestingDoesNotCrash(t *testing.T) {
	deep := strings.Repeat("[", 50000) + strings.Repeat("]", 50000)
	if _, err := Parse([]byte(`{"type":"a","data":{"x":`+deep+`}}`), 5); err == nil {
		t.Fatal("expected an error for absurd nesting")
	}
}

func TestCloudEventsRoundTripDeterministic(t *testing.T) {
	at := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	in := []Entry{
		{Seq: 1, At: at, Type: "fleet.robot.lost", Severity: "critical", Source: "Site A / Zone 4", Key: "k1", Data: map[string]any{"batt": 9.0}},
		{Seq: 2, At: at.Add(time.Second), Type: "fleet.ot.ids", Severity: "warning", Source: "s", Data: map[string]any{}},
	}
	enc := func() string {
		var b bytes.Buffer
		ces := ToCloudEvents(in)
		fmt.Fprintf(&b, "%+v", ces)
		return b.String()
	}
	first, second := enc(), enc()
	if first != second {
		t.Fatal("export not deterministic")
	}
	ces := ToCloudEvents(in)
	if ces[0].SpecVersion != "1.0" || ces[0].ID != "k1" || ces[1].ID != "seq-2" {
		t.Fatalf("%+v", ces)
	}
	if strings.ContainsAny(ces[0].Source, " ") {
		t.Fatalf("source must be URI-safe: %q", ces[0].Source)
	}
	var raw bytes.Buffer
	raw.WriteString("[")
	for i, c := range ces {
		if i > 0 {
			raw.WriteString(",")
		}
		fmt.Fprintf(&raw, `{"specversion":%q,"id":%q,"source":%q,"type":%q,"time":%q,"severity":%q,"data":{}}`, c.SpecVersion, c.ID, c.Source, c.Type, c.Time, c.Severity)
	}
	raw.WriteString("]")
	back, err := Parse(raw.Bytes(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if back[0].Source != in[0].Source || back[0].Key != "k1" || !back[0].At.Equal(at) || back[1].Severity != "warning" {
		t.Fatalf("round trip: %+v", back)
	}
}

func TestWriteNDJSONOneLinePerEntry(t *testing.T) {
	var b bytes.Buffer
	if err := WriteNDJSON(&b, []Entry{{Type: "a", Data: map[string]any{}}, {Type: "b", Data: map[string]any{}}}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(b.String(), "\n") != 2 {
		t.Fatalf("%q", b.String())
	}
}
