// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package device_test

import (
	"path/filepath"
	"testing"

	"github.com/zyvorai/relay-edge/internal/device"
)

func TestPutGet(t *testing.T) {
	s, err := device.Open(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Put(device.Device{ID: "d1", Name: "Pump", ZoneID: "z1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "d1" {
		t.Fatalf("%+v", out)
	}
	got, err := s.Get("d1")
	if err != nil || got.Name != "Pump" {
		t.Fatalf("%+v %v", got, err)
	}
}
