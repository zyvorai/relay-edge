// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func decode(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return m
}

func publishOne(s *Server, evType, sev string, data map[string]any) {
	_ = s.emit("sim", evType, sev, "src", s.simKey("t"), data)
}

func TestJournalStatsEndpoint(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	publishOne(s, "fleet.a", "info", nil)
	publishOne(s, "fleet.a", "critical", nil)
	publishOne(s, "fw.b", "info", nil)
	code, body := do(t, s, "GET", "/v1/journal/stats", "")
	st := decode(t, body)["stats"].(map[string]any)
	if code != 200 || st["count"].(float64) != 3 || st["by_type"].(map[string]any)["fleet.a"].(float64) != 2 ||
		st["by_outcome"].(map[string]any)["published"].(float64) != 3 || st["by_origin"].(map[string]any)["sim"].(float64) != 3 {
		t.Fatalf("%d %s", code, body)
	}
	_, body = do(t, s, "GET", "/v1/journal/stats?types=fleet.*&severities=critical", "")
	if decode(t, body)["stats"].(map[string]any)["count"].(float64) != 1 {
		t.Fatal(body)
	}
	_, body = do(t, s, "GET", "/v1/journal/stats?after=2", "")
	if decode(t, body)["stats"].(map[string]any)["count"].(float64) != 1 {
		t.Fatal(body)
	}
	if code, _ := do(t, s, "GET", "/v1/journal/stats?after=x", ""); code != 400 {
		t.Fatalf("bad after: %d", code)
	}
	_, body = do(t, s, "GET", "/v1/journal/stats?origin=replay", "")
	if decode(t, body)["stats"].(map[string]any)["count"].(float64) != 0 {
		t.Fatal(body)
	}
}

func TestResendWithOverrides(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	publishOne(s, "fleet.robot.lost", "warning", map[string]any{"batt": 9.0, "pose": map[string]any{"x": 1.0, "y": 2.0}, "drop": "me"})
	before := m.count()
	code, body := do(t, s, "POST", "/v1/journal/resend",
		`{"seq":1,"severity":"critical","set":{"pose.x":5,"extra.flag":true},"remove":["drop"],"idempotency_key":"fixed-1"}`)
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	r := decode(t, body)
	data := r["data"].(map[string]any)
	if data["pose"].(map[string]any)["x"] != 5.0 || data["pose"].(map[string]any)["y"] != 2.0 || data["drop"] != nil || data["extra"].(map[string]any)["flag"] != true {
		t.Fatalf("data %v", data)
	}
	res := r["results"].([]any)[0].(map[string]any)
	if res["idempotency_key"] != "fixed-1" || res["outcome"] != "published" || r["severity"] != "critical" || m.count() != before+1 {
		t.Fatalf("%s", body)
	}
	// Journaled with its own origin; original untouched.
	_, jb := do(t, s, "GET", "/v1/journal?origin=resend", "")
	items := decode(t, jb)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["idempotency_key"] != "fixed-1" {
		t.Fatalf("%s", jb)
	}
	_, jb = do(t, s, "GET", "/v1/journal?origin=sim", "")
	if d := decode(t, jb)["items"].([]any)[0].(map[string]any)["data"].(map[string]any); d["batt"] != 9.0 || d["drop"] != "me" {
		t.Fatalf("original mutated: %v", d)
	}
}

func TestResendCountKeysAndDedupKey(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	publishOne(s, "a.b", "info", map[string]any{"v": 1.0})
	code, body := do(t, s, "POST", "/v1/journal/resend", `{"seq":1,"count":3,"idempotency_key":"dup"}`)
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	var keys []string
	for _, r := range decode(t, body)["results"].([]any) {
		keys = append(keys, r.(map[string]any)["idempotency_key"].(string))
	}
	if strings.Join(keys, ",") != "dup-1,dup-2,dup-3" {
		t.Fatalf("%v", keys)
	}
	// No explicit key: seeded keys are deterministic and distinct.
	do(t, s, "POST", "/v1/sim/seed", `{"seed":5}`)
	_, b1 := do(t, s, "POST", "/v1/journal/resend", `{"seq":1,"count":2}`)
	r := decode(t, b1)["results"].([]any)
	k0, k1 := r[0].(map[string]any)["idempotency_key"], r[1].(map[string]any)["idempotency_key"]
	if k0 == k1 || !strings.HasPrefix(k0.(string), "resend/1/s5-") {
		t.Fatalf("%v %v", k0, k1)
	}
}

func TestResendTypeOverrideAndChaosApplies(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	publishOne(s, "a.b", "info", nil)
	do(t, s, "POST", "/v1/chaos/config", `{"enabled":true,"seed":1,"drop_pct":100}`)
	code, body := do(t, s, "POST", "/v1/journal/resend", `{"seq":1,"type":"c.d"}`)
	r := decode(t, body)
	if code != 200 || r["type"] != "c.d" || r["results"].([]any)[0].(map[string]any)["outcome"] != "dropped" {
		t.Fatalf("%d %s", code, body)
	}
	if m.count() != 1 {
		t.Fatalf("dropped resend reached relay: %d", m.count())
	}
}

func TestResendRejects(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	publishOne(s, "a.b", "info", map[string]any{"s": "x"})
	cases := []struct {
		body string
		code int
	}{
		{`{"seq":99}`, 404},
		{`{"seq":0}`, 404},
		{`{"seq":1,"count":101}`, 400},
		{`{"seq":1,"count":-1}`, 400},
		{`{"seq":1,"type":"../admin"}`, 400},
		{`{"seq":1,"type":"a?x=1"}`, 400},
		{`{"seq":1,"severity":"CRIT!"}`, 400},
		{`{"seq":1,"set":{"a..b":1}}`, 400},
		{`{"seq":1,"set":{"s.x":1}}`, 400},
		{`{"seq":1,"idempotency_key":"a\nb"}`, 400},
		{`{"seq":1,"bogus":1}`, 400},
		{`[`, 400},
		{``, 400},
	}
	for _, c := range cases {
		if code, b := do(t, s, "POST", "/v1/journal/resend", c.body); code != c.code {
			t.Errorf("%q: got %d want %d (%s)", c.body, code, c.code, b)
		}
	}
	if m.count() != 1 {
		t.Fatalf("rejected requests published: %d", m.count())
	}
}

func TestResendRelayFailureIs502(t *testing.T) {
	s := newSimPublishServer(t, "http://127.0.0.1:1")
	publishOne(s, "a.b", "info", nil)
	code, body := do(t, s, "POST", "/v1/journal/resend", `{"seq":1}`)
	if code != 502 || decode(t, body)["failed"].(float64) != 1 {
		t.Fatalf("%d %s", code, body)
	}
}

func TestReplayRateAndShuffleDryRun(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	ev := `[{"type":"a.b","severity":"info","data":{"i":0}},{"type":"a.b","severity":"info","data":{"i":1}},{"type":"a.b","severity":"info","data":{"i":2}},{"type":"a.b","severity":"info","data":{"i":3}}]`
	code, body := do(t, s, "POST", "/v1/replay", `{"events":`+ev+`,"dry_run":true,"rate_per_sec":20,"run_id":"r1"}`)
	r := decode(t, body)
	plan := r["plan"].([]any)
	if code != 200 || r["status"].(map[string]any)["pacing"] != "rate" || plan[1].(map[string]any)["delay_ns"].(float64) != 50e6 {
		t.Fatalf("%d %s", code, body)
	}
	code, body = do(t, s, "POST", "/v1/replay", `{"events":`+ev+`,"dry_run":true,"rate_stages":[{"duration_ms":1000,"rate_per_sec":10},{"duration_ms":1000,"rate_per_sec":50}],"start_rate":10,"shuffle_window":4,"shuffle_seed":2,"run_id":"r2"}`)
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	plan = decode(t, body)["plan"].([]any)
	keys := map[string]bool{}
	for _, p := range plan {
		keys[p.(map[string]any)["idempotency_key"].(string)] = true
	}
	if len(plan) != 4 || len(keys) != 4 || !keys["replay/r2/1"] || !keys["replay/r2/4"] {
		t.Fatalf("%s", body)
	}
	// NDJSON body with query params.
	code, body = do(t, s, "POST", "/v1/replay?dry_run=true&rate_per_sec=10&shuffle_window=2&shuffle_seed=1", `{"type":"a.b","severity":"info","data":{}}`+"\n"+`{"type":"a.c","severity":"info","data":{}}`, "Content-Type", "application/x-ndjson")
	if code != 200 || decode(t, body)["status"].(map[string]any)["pacing"] != "rate" {
		t.Fatalf("%d %s", code, body)
	}
	if m.count() != 0 {
		t.Fatal("dry run published")
	}
}

func TestReplayRateRejects(t *testing.T) {
	s := newSimPublishServer(t, "http://127.0.0.1:1")
	ev := `[{"type":"a.b","severity":"info","data":{}}]`
	for _, extra := range []string{
		`"rate_per_sec":5,"speed":2`,
		`"rate_per_sec":5000`,
		`"rate_per_sec":-1`,
		`"rate_stages":[{"duration_ms":0,"rate_per_sec":5}]`,
		`"rate_stages":[{"duration_ms":100,"rate_per_sec":5}],"rate_per_sec":5`,
		`"shuffle_window":1`,
	} {
		if code, b := do(t, s, "POST", "/v1/replay", `{"events":`+ev+`,"dry_run":true,`+extra+`}`); code != 400 {
			t.Errorf("%s: %d %s", extra, code, b)
		}
	}
}

func TestReplayRateActuallyPublishes(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	ev := `[{"type":"a.b","severity":"info","data":{}},{"type":"a.b","severity":"info","data":{}},{"type":"a.b","severity":"info","data":{}}]`
	code, body := do(t, s, "POST", "/v1/replay", `{"events":`+ev+`,"rate_per_sec":500}`)
	if code != 202 {
		t.Fatalf("%d %s", code, body)
	}
	s.pipe.Replay.Wait()
	if m.count() != 3 || s.pipe.Replay.Status().Sent != 3 {
		t.Fatalf("count %d status %+v", m.count(), s.pipe.Replay.Status())
	}
}

func TestChaosTypesScopeThroughEmit(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	if code, b := do(t, s, "POST", "/v1/chaos/config", `{"enabled":true,"seed":1,"drop_pct":100,"types":["fleet.*"]}`); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	publishOne(s, "fleet.x", "info", nil)
	publishOne(s, "fw.y", "info", nil)
	if m.count() != 1 {
		t.Fatalf("relay saw %d, want only the out-of-scope event", m.count())
	}
	_, b := do(t, s, "GET", "/v1/chaos", "")
	st := decode(t, b)["stats"].(map[string]any)
	if st["dropped"].(float64) != 1 || st["skipped"].(float64) != 1 {
		t.Fatalf("%s", b)
	}
	for _, bad := range []string{`{"enabled":true,"types":["../x"]}`, `{"enabled":true,"correlation_pct":101}`} {
		if code, _ := do(t, s, "POST", "/v1/chaos/config", bad); code != 400 {
			t.Fatalf("%s: %d", bad, code)
		}
	}
	if code, _ := do(t, s, "POST", "/v1/chaos/config", `{"enabled":true,"seed":2,"drop_pct":30,"correlation_pct":80}`); code != 200 {
		t.Fatal(code)
	}
}
