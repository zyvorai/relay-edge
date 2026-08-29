// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package relaypub_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zyvorai/relay-edge/internal/relaypub"
)

func TestPublishDirect(t *testing.T) {
	var gotAuth, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"event":{"id":"evt_1"}}`))
	}))
	defer srv.Close()

	c := &relaypub.Client{
		RelayBase:  srv.URL,
		RelayToken: "tok",
		HTTP:       srv.Client(),
	}
	res, err := c.PublishEventType("crop.advisory", "info", "test", "idemp-1", map[string]any{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != "relay" || res.EventID != "evt_1" {
		t.Fatalf("result %#v", res)
	}
	if gotPath != "/v1/events" || gotAuth != "Bearer tok" {
		t.Fatalf("path=%s auth=%s", gotPath, gotAuth)
	}
	if body["type"] != "crop.advisory" {
		t.Fatalf("body %#v", body)
	}
}

func TestPublishGateway(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"messageIds":["m1"]}`))
	}))
	defer srv.Close()

	c := &relaypub.Client{
		GatewayBase:  srv.URL,
		GatewayToken: "gw",
		Project:      "proj",
		HTTP:         srv.Client(),
	}
	res, err := c.PublishEventType("fleet.power.island", "critical", "edge", "k2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != "gateway" {
		t.Fatalf("path %s", res.Path)
	}
	if gotAuth != "Bearer gw" {
		t.Fatalf("auth %s", gotAuth)
	}
	if !strings.Contains(gotPath, "/v1/projects/proj/topics/fleet.power.island:publish") {
		t.Fatalf("path %s", gotPath)
	}
}

func TestPublishGatewayCreatesMissingTopic(t *testing.T) {
	var puts, posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, ":publish"):
			posts++
			if puts == 0 {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"error":"not found"}`))
				return
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"messageIds":["m2"]}`))
		case r.Method == http.MethodPut:
			puts++
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"name":"ok"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := &relaypub.Client{
		GatewayBase: srv.URL,
		Project:     "proj",
		HTTP:        srv.Client(),
	}
	res, err := c.PublishEventType("crop.advisory", "info", "edge", "k3", map[string]any{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != "gateway" {
		t.Fatalf("path %s", res.Path)
	}
	if puts != 1 || posts != 2 {
		t.Fatalf("puts=%d posts=%d", puts, posts)
	}
}
