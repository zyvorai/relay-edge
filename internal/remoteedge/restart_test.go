// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package remoteedge_test

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zyvorai/relay-edge/internal/remoteedge"
)

// A goroutine from a stopped run must exit even if Start is called again
// before it reaches its select (it must not adopt the new stop channel).
func TestStartStopStartDoesNotLeakTicker(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	var calls int32
	e := remoteedge.New(func(remoteedge.Snapshot) {
		if atomic.AddInt32(&calls, 1) == 1 {
			close(entered)
			<-release
		}
	})
	base := runtime.NumGoroutine()
	e.Start()
	<-entered // first goroutine is parked inside onStep
	e.Stop()
	e.Start() // second goroutine
	close(release)
	time.Sleep(200 * time.Millisecond)
	if got := runtime.NumGoroutine(); got != base+1 {
		t.Fatalf("goroutines = %d, want %d (stale ticker leaked)", got, base+1)
	}
	e.Stop()
}
