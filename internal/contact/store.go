// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package contact

import (
	"errors"
	"sync"
	"time"

	"github.com/zyvorai/relay-edge/internal/jsonstore"
)

var ErrNotFound = errors.New("contact not found")

// Contact holds notify endpoints for a person / role.
type Contact struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Role      string            `json:"role,omitempty"` // farmer|operator|agronomist
	FCMToken  string            `json:"fcm_token,omitempty"`
	SMS       string            `json:"sms,omitempty"`
	Email     string            `json:"email,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	byID map[string]Contact
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, byID: map[string]Contact{}}
	var items []Contact
	if err := jsonstore.LoadSlice(path, &items); err != nil {
		return nil, err
	}
	for _, it := range items {
		s.byID[it.ID] = it
	}
	return s, nil
}

func (s *Store) persist() error {
	items := make([]Contact, 0, len(s.byID))
	for _, it := range s.byID {
		items = append(items, it)
	}
	return jsonstore.SaveSlice(s.path, items)
}

func (s *Store) List() []Contact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Contact, 0, len(s.byID))
	for _, it := range s.byID {
		out = append(out, it)
	}
	return out
}

func (s *Store) Get(id string) (Contact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.byID[id]
	if !ok {
		return Contact{}, ErrNotFound
	}
	return it, nil
}

func (s *Store) Put(it Contact) (Contact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it.ID == "" || it.Name == "" {
		return Contact{}, errors.New("id and name required")
	}
	now := time.Now().UTC()
	prev, ok := s.byID[it.ID]
	if ok {
		it.CreatedAt = prev.CreatedAt
	} else {
		it.CreatedAt = now
	}
	if it.Labels == nil {
		it.Labels = map[string]string{}
	}
	it.UpdatedAt = now
	s.byID[it.ID] = it
	if err := s.persist(); err != nil {
		return Contact{}, err
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
