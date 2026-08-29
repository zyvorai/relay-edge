// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package contact_test

import (
	"path/filepath"
	"testing"

	"github.com/zyvorai/relay-edge/internal/contact"
)

func TestPutGet(t *testing.T) {
	s, err := contact.Open(filepath.Join(t.TempDir(), "contacts.json"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Put(contact.Contact{ID: "c1", Name: "Op", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "c1" {
		t.Fatalf("%+v", out)
	}
	got, err := s.Get("c1")
	if err != nil || got.Role != "operator" {
		t.Fatalf("%+v %v", got, err)
	}
}
