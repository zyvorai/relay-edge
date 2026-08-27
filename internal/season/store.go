// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package season

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("season not found")
	ErrConflict = errors.New("season conflict")
)

// Season is an edge growing-season / crop-window document.
// Relay does not own seasons — relay-edge stores them and publishes events into Relay.
// Valid growth stages for calendar advisories.
var ValidStages = map[string]bool{
	"sowing": true, "vegetative": true, "flowering": true,
	"fruiting": true, "harvest": true, "idle": true, "": true,
}

type Season struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Crop        string            `json:"crop,omitempty"`
	SiteID      string            `json:"site_id,omitempty"`
	Site        string            `json:"site,omitempty"` // display name (denormalized)
	Stage       string            `json:"stage,omitempty"` // sowing|vegetative|flowering|fruiting|harvest|idle
	TenantHint  string            `json:"tenant_hint,omitempty"`
	Status      string            `json:"status"` // planned | active | closed
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      time.Time         `json:"ends_at"`
	Labels      map[string]string `json:"labels,omitempty"`
	Notes       string            `json:"notes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	OpenedAt    *time.Time        `json:"opened_at,omitempty"`
	ClosedAt    *time.Time        `json:"closed_at,omitempty"`
	LastEventID string            `json:"last_event_id,omitempty"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	byID map[string]Season
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, byID: map[string]Season{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var items []Season
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	for _, it := range items {
		s.byID[it.ID] = it
	}
	return nil
}

func (s *Store) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	items := make([]Season, 0, len(s.byID))
	for _, it := range s.byID {
		items = append(items, it)
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) List() []Season {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Season, 0, len(s.byID))
	for _, it := range s.byID {
		out = append(out, it)
	}
	return out
}

func (s *Store) Get(id string) (Season, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.byID[id]
	if !ok {
		return Season{}, ErrNotFound
	}
	return it, nil
}

func (s *Store) Put(it Season) (Season, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if it.ID == "" {
		return Season{}, errors.New("id required")
	}
	if it.Name == "" {
		return Season{}, errors.New("name required")
	}
	if it.Status == "" {
		it.Status = "planned"
	}
	if !ValidStages[it.Stage] {
		return Season{}, errors.New("invalid stage")
	}
	prev, ok := s.byID[it.ID]
	if ok {
		it.CreatedAt = prev.CreatedAt
		it.OpenedAt = prev.OpenedAt
		it.ClosedAt = prev.ClosedAt
		it.LastEventID = prev.LastEventID
		if it.Stage == "" {
			it.Stage = prev.Stage
		}
		if it.SiteID == "" {
			it.SiteID = prev.SiteID
		}
	} else if it.CreatedAt.IsZero() {
		it.CreatedAt = now
	}
	it.UpdatedAt = now
	if it.Labels == nil {
		it.Labels = map[string]string{}
	}
	s.byID[it.ID] = it
	if err := s.persist(); err != nil {
		return Season{}, err
	}
	return it, nil
}

func (s *Store) UpdateStage(id, stage string) (Season, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !ValidStages[stage] || stage == "" {
		return Season{}, errors.New("invalid stage")
	}
	it, ok := s.byID[id]
	if !ok {
		return Season{}, ErrNotFound
	}
	it.Stage = stage
	it.UpdatedAt = time.Now().UTC()
	s.byID[id] = it
	if err := s.persist(); err != nil {
		return Season{}, err
	}
	return it, nil
}

func (s *Store) UpdateStatus(id, status string, eventID string) (Season, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.byID[id]
	if !ok {
		return Season{}, ErrNotFound
	}
	now := time.Now().UTC()
	it.Status = status
	it.UpdatedAt = now
	if eventID != "" {
		it.LastEventID = eventID
	}
	switch status {
	case "active":
		it.OpenedAt = &now
	case "closed":
		it.ClosedAt = &now
	}
	s.byID[id] = it
	if err := s.persist(); err != nil {
		return Season{}, err
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
