// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type relayMock struct {
	mu   sync.Mutex
	got  []map[string]any
	keys []string
	srv  *httptest.Server
}

func newRelayMock(t *testing.T) *relayMock {
	t.Helper()
	m := &relayMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var v map[string]any
		_ = json.Unmarshal(b, &v)
		m.mu.Lock()
		m.got = append(m.got, v)
		if k, ok := v["idempotency_key"].(string); ok {
			m.keys = append(m.keys, k)
		}
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"event":{"id":"evt_1"}}`))
	}))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *relayMock) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.got)
}

func do(t *testing.T, s *Server, method, path, body string, hdr ...string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestSimSeedMakesFleetDeterministic(t *testing.T) {
	s := newSimPublishServer(t, "http://127.0.0.1:1")
	run := func(seed string) []map[string]float64 {
		if code, b := do(t, s, "POST", "/v1/sim/seed", `{"seed":`+seed+`}`); code != 200 {
			t.Fatalf("%d %s", code, b)
		}
		var out []map[string]float64
		for i := 0; i < 4; i++ {
			out = append(out, s.Fleet.Tick().Values)
		}
		return out
	}
	a, b, c := run("42"), run("42"), run("43")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed must give the same fleet values")
	}
	if reflect.DeepEqual(a, c) {
		t.Fatal("different seeds must differ")
	}
}

func TestSimSeedDeterministicRemoteEdgeAndFirewater(t *testing.T) {
	s := newSimPublishServer(t, "http://127.0.0.1:1")
	do(t, s, "POST", "/v1/sim/seed", `{"seed":7}`)
	re1, fw1 := s.RemoteEdge.Tick().Values, s.FW.Tick().Values
	do(t, s, "POST", "/v1/sim/seed", `{"seed":7}`)
	re2, fw2 := s.RemoteEdge.Tick().Values, s.FW.Tick().Values
	if !reflect.DeepEqual(re1, re2) {
		t.Fatal("remote-edge not deterministic")
	}
	if !reflect.DeepEqual(fw1, fw2) {
		t.Fatal("firewater not deterministic")
	}
}

func TestSimSeedEndpointHostileInput(t *testing.T) {
	s := newSimPublishServer(t, "http://127.0.0.1:1")
	for _, body := range []string{`{"seed":"x"}`, `{"seed":1.5}`, `{"seed":99999999999999999999}`, `[`, ``} {
		if code, _ := do(t, s, "POST", "/v1/sim/seed", body); code != 400 {
			t.Fatalf("body %q: code %d", body, code)
		}
	}
	do(t, s, "POST", "/v1/sim/seed", `{"seed":-5}`)
	if _, b := do(t, s, "GET", "/v1/sim/seed", ""); !strings.Contains(b, `"seed":-5`) {
		t.Fatal(b)
	}
	do(t, s, "POST", "/v1/sim/seed", `{"seed":null}`)
	if _, b := do(t, s, "GET", "/v1/sim/seed", ""); !strings.Contains(b, `"seeded":false`) {
		t.Fatal(b)
	}
}

func TestSeededKeysAreDeterministic(t *testing.T) {
	s := newSimPublishServer(t, "http://127.0.0.1:1")
	do(t, s, "POST", "/v1/sim/seed", `{"seed":9}`)
	k1, k2 := s.simKey("p"), s.simKey("p")
	do(t, s, "POST", "/v1/sim/seed", `{"seed":9}`)
	if s.simKey("p") != k1 || k1 == k2 {
		t.Fatalf("keys %q %q", k1, k2)
	}
	do(t, s, "POST", "/v1/sim/seed", `{"seed":null}`)
	if strings.Contains(s.simKey("p"), "/s9-") {
		t.Fatal("unseeded key used seed format")
	}
}

func seeded(t *testing.T, m *relayMock) *Server {
	t.Helper()
	s := newSimPublishServer(t, m.srv.URL)
	if code, b := do(t, s, "POST", "/v1/firewater/seed", ""); code != 200 {
		t.Fatalf("seed %d %s", code, b)
	}
	return s
}

func TestEmitJournalsPublishedEvent(t *testing.T) {
	m := newRelayMock(t)
	s := seeded(t, m)
	s.publishSimEvent("fleet.robot.lost", "critical", "amr.relocalize", "amr_pose", "fleet", map[string]any{"batt": 9})
	if m.count() != 1 {
		t.Fatalf("published %d", m.count())
	}
	items := s.pipe.Journal.List(0, 10)
	if len(items) != 1 || items[0].Type != "fleet.robot.lost" || items[0].Outcome != "published" || items[0].Origin != "sim" {
		t.Fatalf("%+v", items)
	}
	if items[0].Data["sim_domain"] != "fleet" {
		t.Fatalf("journal must hold the stamped payload: %+v", items[0].Data)
	}
}

func TestChaosDropDuplicateError(t *testing.T) {
	m := newRelayMock(t)
	s := seeded(t, m)
	cfg := func(body string) {
		t.Helper()
		if code, b := do(t, s, "POST", "/v1/chaos/config", body); code != 200 {
			t.Fatalf("%d %s", code, b)
		}
	}
	cfg(`{"enabled":true,"seed":1,"drop_pct":100}`)
	s.publishSimEvent("a.b", "info", "", "d", "fleet", nil)
	if m.count() != 0 || s.pipe.Journal.List(0, 5)[0].Outcome != "dropped" {
		t.Fatal("drop failed")
	}
	cfg(`{"enabled":true,"seed":1,"duplicate_pct":100}`)
	s.publishSimEvent("a.b", "info", "", "d", "fleet", nil)
	if m.count() != 2 || m.keys[0] != m.keys[1] {
		t.Fatalf("duplicate: %d %v", m.count(), m.keys)
	}
	cfg(`{"enabled":true,"seed":1,"error_pct":100}`)
	s.publishSimEvent("a.b", "info", "", "d", "fleet", nil)
	if m.count() != 2 {
		t.Fatal("error action must not publish")
	}
	last := s.pipe.Journal.List(0, 10)
	if last[len(last)-1].Outcome != "error" {
		t.Fatalf("%+v", last[len(last)-1])
	}
	_, body := do(t, s, "GET", "/v1/chaos", "")
	if !strings.Contains(body, `"dropped":0`) || !strings.Contains(body, `"errored":1`) {
		t.Fatalf("stats reset on reconfigure expected: %s", body)
	}
	do(t, s, "POST", "/v1/chaos/reset", "")
	s.publishSimEvent("a.b", "info", "", "d", "fleet", nil)
	if m.count() != 3 {
		t.Fatal("reset must restore normal publishing")
	}
}

func TestChaosDelayPublishesLater(t *testing.T) {
	m := newRelayMock(t)
	s := seeded(t, m)
	do(t, s, "POST", "/v1/chaos/config", `{"enabled":true,"delay_ms":80}`)
	s.publishSimEvent("a.b", "info", "", "d", "fleet", nil)
	if m.count() != 0 {
		t.Fatal("must not publish synchronously")
	}
	deadline := time.Now().Add(3 * time.Second)
	for m.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if m.count() != 1 {
		t.Fatal("delayed publish never arrived")
	}
}

func TestChaosEndpointHostileInput(t *testing.T) {
	s := newSimPublishServer(t, "http://127.0.0.1:1")
	for _, body := range []string{
		`{"drop_pct":-1}`, `{"drop_pct":101}`, `{"delay_ms":10001}`, `{"jitter_ms":-3}`,
		`{"unknown":1}`, `{"drop_pct":"a"}`, `{"seed":1.5}`, `[`, ``,
	} {
		if code, _ := do(t, s, "POST", "/v1/chaos/config", body); code != 400 {
			t.Fatalf("body %q: code %d", body, code)
		}
	}
}

func TestJournalEndpointsAndExport(t *testing.T) {
	m := newRelayMock(t)
	s := seeded(t, m)
	for i := 0; i < 3; i++ {
		s.publishSimEvent("a.b", "info", "", "d", "fleet", map[string]any{"i": i})
	}
	_, b := do(t, s, "GET", "/v1/journal?after=1&limit=1", "")
	var lr struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal([]byte(b), &lr)
	if len(lr.Items) != 1 || lr.Items[0]["seq"].(float64) != 2 {
		t.Fatal(b)
	}
	code, nd := do(t, s, "GET", "/v1/journal/export", "")
	if code != 200 || strings.Count(nd, "\n") != 3 {
		t.Fatalf("%d %q", code, nd)
	}
	_, ce := do(t, s, "GET", "/v1/journal/export?format=cloudevents", "")
	if !strings.Contains(ce, `"specversion":"1.0"`) {
		t.Fatal(ce)
	}
	for _, bad := range []string{"?format=xml", "?from=abc", "?limit=-1", "?after=-1"} {
		p := "/v1/journal/export" + bad
		if strings.HasPrefix(bad, "?after") {
			p = "/v1/journal" + bad
		}
		if code, _ := do(t, s, "GET", p, ""); code != 400 {
			t.Fatalf("%s -> %d", p, code)
		}
	}
	if _, b := do(t, s, "POST", "/v1/journal/clear", ""); !strings.Contains(b, `"cleared":3`) {
		t.Fatal(b)
	}
}

func TestReplayFromJournalIsIdempotentAcrossRuns(t *testing.T) {
	m := newRelayMock(t)
	s := seeded(t, m)
	for i := 0; i < 3; i++ {
		s.publishSimEvent("a.b", "info", "", "d", "fleet", map[string]any{"i": i})
	}
	base := m.count()
	body := `{"from_journal":true,"origin":"sim","speed":1000,"run_id":"golden"}`
	waitReplay := func() {
		s.pipe.Replay.Wait()
	}
	code, b := do(t, s, "POST", "/v1/replay", body)
	if code != 202 {
		t.Fatalf("%d %s", code, b)
	}
	waitReplay()
	if m.count() != base+3 {
		t.Fatalf("published %d", m.count()-base)
	}
	do(t, s, "POST", "/v1/replay", body)
	waitReplay()
	k := m.keys[base:]
	if len(k) != 6 || k[0] != k[3] || k[1] != k[4] || k[0] != "replay/golden/1" {
		t.Fatalf("replays must reuse keys: %v", k)
	}
	if st := s.pipe.Replay.Status(); st.Sent != 3 || st.State != "done" {
		t.Fatalf("%+v", st)
	}
}

func TestReplayInlineAndNDJSONBody(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	code, b := do(t, s, "POST", "/v1/replay", `{"events":[{"type":"x.y","data":{"a":1}},{"type":"x.z"}],"speed":1000}`)
	if code != 202 {
		t.Fatalf("%d %s", code, b)
	}
	s.pipe.Replay.Wait()
	code, b = do(t, s, "POST", "/v1/replay?speed=1000&run_id=nd", "{\"type\":\"p.q\"}\n{\"type\":\"p.r\"}\n", "Content-Type", "application/x-ndjson")
	if code != 202 {
		t.Fatalf("%d %s", code, b)
	}
	s.pipe.Replay.Wait()
	if m.count() != 4 {
		t.Fatalf("count %d", m.count())
	}
}

func TestReplayDryRunAndHostile(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	code, b := do(t, s, "POST", "/v1/replay", `{"events":[{"type":"x.y"}],"dry_run":true}`)
	if code != 200 || !strings.Contains(b, `"plan"`) || m.count() != 0 {
		t.Fatalf("%d %s", code, b)
	}
	cases := []struct {
		name, body string
		want       int
	}{
		{"no events", `{}`, 400},
		{"empty list", `{"events":[]}`, 400},
		{"path type", `{"events":[{"type":"../../v1/admin"}]}`, 400},
		{"bad speed", `{"events":[{"type":"a"}],"speed":5000}`, 400},
		{"bad run id", `{"events":[{"type":"a"}],"run_id":"a/b"}`, 400},
		{"bad gap", `{"events":[{"type":"a"}],"max_gap_ms":999999}`, 400},
		{"data array", `{"events":[{"type":"a","data":[1]}]}`, 400},
		{"invalid json", `{"events":`, 400},
		{"too many", `{"events":[{"type":"a"},{"type":"b"}],"max_events":1}`, 413},
		{"empty journal", `{"from_journal":true}`, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, b := do(t, s, "POST", "/v1/replay", c.body); code != c.want {
				t.Fatalf("%d %s", code, b)
			}
		})
	}
	if m.count() != 0 {
		t.Fatal("hostile input must not publish")
	}
}

func TestReplayBusyAndCancel(t *testing.T) {
	m := newRelayMock(t)
	s := newSimPublishServer(t, m.srv.URL)
	// 2 events 1h apart, speed 0.01 with max gap 30s => the runner blocks in sleep
	body := `{"events":[{"type":"a","at":"2026-01-01T00:00:00Z"},{"type":"b","at":"2026-01-01T01:00:00Z"}],"speed":0.01,"max_gap_ms":30000}`
	if code, b := do(t, s, "POST", "/v1/replay", body); code != 202 {
		t.Fatalf("%d %s", code, b)
	}
	if code, _ := do(t, s, "POST", "/v1/replay", body); code != 409 {
		t.Fatalf("second run code %d", code)
	}
	if _, b := do(t, s, "POST", "/v1/replay/cancel", ""); !strings.Contains(b, `"cancelled":true`) {
		t.Fatal(b)
	}
	s.pipe.Replay.Wait()
	if _, b := do(t, s, "GET", "/v1/replay", ""); !strings.Contains(b, `"state":"cancelled"`) {
		t.Fatal(b)
	}
}

func TestReplayThroughChaosIsSeededReproducible(t *testing.T) {
	run := func() []string {
		m := newRelayMock(t)
		s := newSimPublishServer(t, m.srv.URL)
		do(t, s, "POST", "/v1/chaos/config", `{"enabled":true,"seed":11,"drop_pct":40}`)
		var evs []string
		for i := 0; i < 30; i++ {
			evs = append(evs, `{"type":"a.b"}`)
		}
		do(t, s, "POST", "/v1/replay", `{"events":[`+strings.Join(evs, ",")+`],"speed":1000,"run_id":"c"}`)
		s.pipe.Replay.Wait()
		m.mu.Lock()
		defer m.mu.Unlock()
		return append([]string(nil), m.keys...)
	}
	a, b := run(), run()
	if len(a) == 0 || len(a) == 30 || !reflect.DeepEqual(a, b) {
		t.Fatalf("seeded chaos replay not reproducible: %d vs %d", len(a), len(b))
	}
}
