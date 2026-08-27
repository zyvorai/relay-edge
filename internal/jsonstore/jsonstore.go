// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package jsonstore

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadSlice reads a JSON array file into out. Missing file → empty.
func LoadSlice[T any](path string, out *[]T) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			*out = nil
			return nil
		}
		return err
	}
	return json.Unmarshal(b, out)
}

// SaveSlice writes items as indented JSON atomically.
func SaveSlice[T any](path string, items []T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
