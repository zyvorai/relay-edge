// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package jsonstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "items.json")
	items := []string{"a", "b"}
	if err := SaveSlice(path, items); err != nil {
		t.Fatal(err)
	}
	var out []string
	if err := LoadSlice(path, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("got %#v", out)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp leftover: %v", err)
	}
}

func TestLoadSliceMissing(t *testing.T) {
	var out []int
	if err := LoadSlice(filepath.Join(t.TempDir(), "missing.json"), &out); err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatalf("expected nil, got %#v", out)
	}
}
