// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater

import (
	"sync"
	"time"
)

// AlarmState is a reduced ISA-18.2 lifecycle.
type AlarmState string

const (
	AlarmNormal   AlarmState = "normal"
	AlarmUnack    AlarmState = "unack"
	AlarmAcked    AlarmState = "acked"
	AlarmShelved  AlarmState = "shelved"
	AlarmReturned AlarmState = "returned" // condition cleared, waiting ack
)

type Alarm struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Severity  Severity   `json:"severity"`
	State     AlarmState `json:"state"`
	Count     int        `json:"count"`
	FirstAt   time.Time  `json:"first_at"`
	LastAt    time.Time  `json:"last_at"`
	ShelvedTo *time.Time `json:"shelved_until,omitempty"`
	Message   string     `json:"message"`
}

type Book struct {
	mu     sync.Mutex
	active map[string]*Alarm
}

func NewBook() *Book { return &Book{active: map[string]*Alarm{}} }

func (b *Book) Ingest(evts []Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now().UTC()
	seen := map[string]bool{}
	for _, ev := range evts {
		if ev.Severity == SevInfo {
			continue
		}
		seen[ev.Type] = true
		a, ok := b.active[ev.Type]
		if !ok {
			b.active[ev.Type] = &Alarm{
				ID: ev.Type, Type: ev.Type, Severity: ev.Severity,
				State: AlarmUnack, Count: 1, FirstAt: now, LastAt: now,
				Message: ev.Command,
			}
			continue
		}
		if a.State == AlarmShelved && a.ShelvedTo != nil && now.Before(*a.ShelvedTo) {
			a.Count++
			a.LastAt = now
			continue
		}
		if a.State == AlarmShelved {
			a.State = AlarmUnack
			a.ShelvedTo = nil
		}
		if a.State == AlarmNormal || a.State == AlarmReturned {
			a.State = AlarmUnack
		}
		a.Count++
		a.LastAt = now
		a.Severity = ev.Severity
	}
	for id, a := range b.active {
		if seen[id] {
			continue
		}
		if a.State == AlarmUnack || a.State == AlarmAcked {
			a.State = AlarmReturned
		}
	}
}

func (b *Book) Ack(id string) *Alarm {
	b.mu.Lock()
	defer b.mu.Unlock()
	a := b.active[id]
	if a == nil {
		return nil
	}
	if a.State == AlarmReturned {
		delete(b.active, id)
		cp := *a
		cp.State = AlarmNormal
		return &cp
	}
	a.State = AlarmAcked
	cp := *a
	return &cp
}

func (b *Book) Shelve(id string, d time.Duration) *Alarm {
	b.mu.Lock()
	defer b.mu.Unlock()
	a := b.active[id]
	if a == nil {
		return nil
	}
	until := time.Now().UTC().Add(d)
	a.State = AlarmShelved
	a.ShelvedTo = &until
	cp := *a
	return &cp
}

func (b *Book) List() []Alarm {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Alarm, 0, len(b.active))
	for _, a := range b.active {
		out = append(out, *a)
	}
	return out
}
