// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"errors"
	"sync"
	"time"

	"github.com/zyvorai/relay-edge/internal/jsonstore"
)

var ErrNotFound = errors.New("device not found")

// Device is a valve / pump / FasalJet / hub bound to a zone.
type Device struct {
	ID         string            `json:"id"`
	ZoneID     string            `json:"zone_id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"` // valve|pump|fasaljet|hub
	ExternalID string            `json:"external_id"`
	Commands   []string          `json:"commands,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Active     bool              `json:"active"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	byID map[string]Device
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, byID: map[string]Device{}}
	var items []Device
	if err := jsonstore.LoadSlice(path, &items); err != nil {
		return nil, err
	}
	for _, it := range items {
		s.byID[it.ID] = it
	}
	return s, nil
}

func (s *Store) persist() error {
	items := make([]Device, 0, len(s.byID))
	for _, it := range s.byID {
		items = append(items, it)
	}
	return jsonstore.SaveSlice(s.path, items)
}

func (s *Store) List(zoneID string) []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Device, 0)
	for _, it := range s.byID {
		if zoneID == "" || it.ZoneID == zoneID {
			out = append(out, it)
		}
	}
	return out
}

func (s *Store) Get(id string) (Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.byID[id]
	if !ok {
		return Device{}, ErrNotFound
	}
	return it, nil
}

func (s *Store) FirstForZone(zoneID string) (Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, it := range s.byID {
		if it.ZoneID == zoneID && it.Active {
			return it, true
		}
	}
	return Device{}, false
}

func (s *Store) Put(it Device) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it.ID == "" || it.ZoneID == "" || it.Name == "" {
		return Device{}, errors.New("id, zone_id, and name required")
	}
	if it.Kind == "" {
		it.Kind = "valve"
	}
	if it.ExternalID == "" {
		it.ExternalID = it.ID
	}
	now := time.Now().UTC()
	prev, ok := s.byID[it.ID]
	if ok {
		it.CreatedAt = prev.CreatedAt
	} else {
		it.CreatedAt = now
		it.Active = true
	}
	if it.Labels == nil {
		it.Labels = map[string]string{}
	}
	if it.Commands == nil {
		it.Commands = []string{}
	}
	it.UpdatedAt = now
	s.byID[it.ID] = it
	if err := s.persist(); err != nil {
		return Device{}, err
	}
	return it, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return ErrNotFound
	}
	delete(s.byID, id)
	return s.persist()
}
