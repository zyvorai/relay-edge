// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package season_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/relay-edge/internal/season"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seasons.json")
	st, err := season.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.Put(season.Season{
		ID: "s1", Name: "Kharif", Crop: "grape", Site: "Farm-1",
		StartsAt: time.Now().UTC(), EndsAt: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "planned" {
		t.Fatalf("status %s", got.Status)
	}
	st2, err := season.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := st2.Get("s1")
	if err != nil || again.Name != "Kharif" {
		t.Fatalf("reload: %+v %v", again, err)
	}
	active, err := st2.UpdateStatus("s1", "active", "evt_1")
	if err != nil || active.Status != "active" || active.LastEventID != "evt_1" {
		t.Fatalf("open: %+v %v", active, err)
	}
}
