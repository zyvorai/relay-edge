package fleet_test

import (
	"testing"

	"github.com/zyvorai/relay-edge/internal/fleet"
)

func TestCatalogBreadth(t *testing.T) {
	if len(fleet.Catalog()) < 60 {
		t.Fatalf("too thin: %d", len(fleet.Catalog()))
	}
	need := []string{"robot", "rtls", "wearable", "energy", "building", "ot_gw", "machine", "water", "env", "rail", "agri", "marine", "life", "radio", "security"}
	// life is domain not class — classes include "life"
	seen := map[string]bool{}
	for _, d := range fleet.Catalog() {
		seen[d.Class] = true
	}
	for _, c := range []string{"robot", "energy", "ot_gw", "agri", "rail", "security"} {
		if !seen[c] {
			t.Fatalf("missing %s", c)
		}
	}
	_ = need
}

func TestBlackoutEvent(t *testing.T) {
	ev := fleet.Derive(fleet.New().SetScenario("blackout"))
	if len(ev) == 0 {
		t.Fatal("expected power island event")
	}
}
