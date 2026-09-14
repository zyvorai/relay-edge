// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
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

// SaveSlice writes items as indented JSON atomically (temp + fsync + rename).
func SaveSlice[T any](path string, items []T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(path, b, 0o644)
}

// WriteAtomic writes data to path via temp file, fsync, and rename.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
