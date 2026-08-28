// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package atlas_test

import (
	"testing"

	"github.com/zyvorai/relay-edge/internal/atlas"
)

func TestOfflineEvents(t *testing.T) {
	e := atlas.New(nil)
	snap := e.SetScenario("offline")
	if snap.LinkMode != "offline" {
		t.Fatalf("link %s", snap.LinkMode)
	}
	ev := atlas.Derive(snap)
	if len(ev) == 0 {
		t.Fatal("expected offline event")
	}
}

func TestCatalogClasses(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range atlas.Catalog() {
		seen[a.Class] = true
	}
	for _, c := range []string{"galleon", "starlink", "sdwan", "p5g", "drone", "vision", "iot"} {
		if !seen[c] {
			t.Fatalf("missing class %s", c)
		}
	}
}
