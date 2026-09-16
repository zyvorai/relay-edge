// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/relay-edge/internal/relaypub"
)

// runStream starts an SSE GET against path, lets it run for a moment (so the
// initial snapshot is written and flushed and any triggerAfterStart side
// effect has a chance to broadcast), then cancels the request context and
// waits for the handler to return before it's safe to read the recorder's
// body (avoids a data race between the handler goroutine still writing and
// the test goroutine reading).
func runStream(t *testing.T, s interface{ Handler() http.Handler }, path string, triggerAfterStart func()) (*httptest.ResponseRecorder, string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(rr, req)
	}()

	time.Sleep(50 * time.Millisecond) // let the initial "data: {snapshot}" line flush
	if triggerAfterStart != nil {
		triggerAfterStart()
		time.Sleep(50 * time.Millisecond) // let the broadcast reach the subscriber channel
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream handler did not return after context cancellation")
	}
	return rr, rr.Body.String()
}

func TestFwStream_InitialSnapshotAndTick(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr, body := runStream(t, s, "/v1/firewater/stream", func() {
		tr := httptest.NewRecorder()
		s.Handler().ServeHTTP(tr, httptest.NewRequest(http.MethodPost, "/v1/firewater/tick", nil))
	})
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(body, `"kind":"snapshot"`) {
		t.Fatalf("stream missing initial snapshot:\n%s", body)
	}
	if !strings.Contains(body, `"kind":"tick"`) {
		t.Fatalf("stream missing broadcast tick event:\n%s", body)
	}
}

func TestFleetStream_InitialSnapshotAndTick(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr, body := runStream(t, s, "/v1/fleet/stream", func() {
		tr := httptest.NewRecorder()
		s.Handler().ServeHTTP(tr, httptest.NewRequest(http.MethodPost, "/v1/fleet/tick", nil))
	})
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(body, `"kind":"snapshot"`) {
		t.Fatalf("stream missing initial snapshot:\n%s", body)
	}
	if !strings.Contains(body, `"kind":"tick"`) {
		t.Fatalf("stream missing broadcast tick event:\n%s", body)
	}
}

func TestRemoteEdgeStream_InitialSnapshotAndTick(t *testing.T) {
	s := testServer(t, nil, &relaypub.Client{RelayBase: "http://127.0.0.1:18080"})
	rr, body := runStream(t, s, "/v1/remote-edge/stream", func() {
		tr := httptest.NewRecorder()
		s.Handler().ServeHTTP(tr, httptest.NewRequest(http.MethodPost, "/v1/remote-edge/tick", nil))
	})
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(body, `"kind":"snapshot"`) {
		t.Fatalf("stream missing initial snapshot:\n%s", body)
	}
	if !strings.Contains(body, `"kind":"tick"`) {
		t.Fatalf("stream missing broadcast tick event:\n%s", body)
	}
}
