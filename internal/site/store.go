// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package site

import (
	"errors"
	"sync"
	"time"

	"github.com/zyvorai/relay-edge/internal/jsonstore"
)

var (
	ErrNotFound = errors.New("site or zone not found")
	ErrConflict = errors.New("site or zone conflict")
)

// VerificationProbe is an HTTP check Relay can run after Act.
type VerificationProbe struct {
	URL      string `json:"url"`
	Method   string `json:"method,omitempty"`
	JSONPath string `json:"json_path,omitempty"`
	Expect   string `json:"expect,omitempty"`
}

// Site is a farm / pack-house location.
type Site struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Active    bool              `json:"active"`
	Lat       *float64          `json:"lat,omitempty"`
	Lon       *float64          `json:"lon,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	// Routing maps role (farmer|operator|agronomist) → contact_id.
	Routing   map[string]string `json:"routing,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Zone is a plot / irrigation block under a site.
type Zone struct {
	ID        string             `json:"id"`
	SiteID    string             `json:"site_id"`
	Name      string             `json:"name"`
	Code      string             `json:"code"` // e.g. A4 — stamped on events as "zone"
	Active    bool               `json:"active"`
	Labels    map[string]string  `json:"labels,omitempty"`
	Telemetry *VerificationProbe `json:"telemetry,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type Store struct {
	mu        sync.RWMutex
	sitesPath string
	zonesPath string
	sites     map[string]Site
	zones     map[string]Zone
}

func Open(sitesPath, zonesPath string) (*Store, error) {
	s := &Store{
		sitesPath: sitesPath,
		zonesPath: zonesPath,
		sites:     map[string]Site{},
		zones:     map[string]Zone{},
	}
	var sites []Site
	if err := jsonstore.LoadSlice(sitesPath, &sites); err != nil {
		return nil, err
	}
	for _, it := range sites {
		s.sites[it.ID] = it
	}
	var zones []Zone
	if err := jsonstore.LoadSlice(zonesPath, &zones); err != nil {
		return nil, err
	}
	for _, it := range zones {
		s.zones[it.ID] = it
	}
	return s, nil
}

func (s *Store) persistSites() error {
	items := make([]Site, 0, len(s.sites))
	for _, it := range s.sites {
		items = append(items, it)
	}
	return jsonstore.SaveSlice(s.sitesPath, items)
}

func (s *Store) persistZones() error {
	items := make([]Zone, 0, len(s.zones))
	for _, it := range s.zones {
		items = append(items, it)
	}
	return jsonstore.SaveSlice(s.zonesPath, items)
}

func (s *Store) ListSites() []Site {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Site, 0, len(s.sites))
	for _, it := range s.sites {
		out = append(out, it)
	}
	return out
}

func (s *Store) GetSite(id string) (Site, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.sites[id]
	if !ok {
		return Site{}, ErrNotFound
	}
	return it, nil
}

func (s *Store) PutSite(it Site) (Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it.ID == "" || it.Name == "" {
		return Site{}, errors.New("id and name required")
	}
	now := time.Now().UTC()
	prev, ok := s.sites[it.ID]
	if ok {
		it.CreatedAt = prev.CreatedAt
	} else {
		it.CreatedAt = now
		it.Active = true
	}
	if it.Labels == nil {
		it.Labels = map[string]string{}
	}
	if it.Routing == nil {
		if ok && prev.Routing != nil {
			it.Routing = prev.Routing
		} else {
			it.Routing = map[string]string{}
		}
	}
	it.UpdatedAt = now
	s.sites[it.ID] = it
	if err := s.persistSites(); err != nil {
		return Site{}, err
	}
	return it, nil
}

func (s *Store) DeleteSite(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sites[id]; !ok {
		return ErrNotFound
	}
	for zid, z := range s.zones {
		if z.SiteID == id {
			delete(s.zones, zid)
		}
	}
	delete(s.sites, id)
	if err := s.persistZones(); err != nil {
		return err
	}
	return s.persistSites()
}

func (s *Store) SetRouting(siteID string, routing map[string]string) (Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.sites[siteID]
	if !ok {
		return Site{}, ErrNotFound
	}
	if routing == nil {
		routing = map[string]string{}
	}
	it.Routing = routing
	it.UpdatedAt = time.Now().UTC()
	s.sites[siteID] = it
	if err := s.persistSites(); err != nil {
		return Site{}, err
	}
	return it, nil
}

func (s *Store) ListZones(siteID string) []Zone {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Zone, 0)
	for _, it := range s.zones {
		if siteID == "" || it.SiteID == siteID {
			out = append(out, it)
		}
	}
	return out
}

func (s *Store) GetZone(id string) (Zone, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.zones[id]
	if !ok {
		return Zone{}, ErrNotFound
	}
	return it, nil
}

func (s *Store) PutZone(it Zone) (Zone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it.ID == "" || it.SiteID == "" || it.Name == "" {
		return Zone{}, errors.New("id, site_id, and name required")
	}
	if _, ok := s.sites[it.SiteID]; !ok {
		return Zone{}, errors.New("site not found")
	}
	if it.Code == "" {
		it.Code = it.Name
	}
	now := time.Now().UTC()
	prev, ok := s.zones[it.ID]
	if ok {
		it.CreatedAt = prev.CreatedAt
		if it.Telemetry == nil {
			it.Telemetry = prev.Telemetry
		}
	} else {
		it.CreatedAt = now
		it.Active = true
	}
	if it.Labels == nil {
		it.Labels = map[string]string{}
	}
	it.UpdatedAt = now
	s.zones[it.ID] = it
	if err := s.persistZones(); err != nil {
		return Zone{}, err
	}
	return it, nil
}

func (s *Store) SetTelemetry(zoneID string, probe *VerificationProbe) (Zone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.zones[zoneID]
	if !ok {
		return Zone{}, ErrNotFound
	}
	it.Telemetry = probe
	it.UpdatedAt = time.Now().UTC()
	s.zones[zoneID] = it
	if err := s.persistZones(); err != nil {
		return Zone{}, err
	}
	return it, nil
}

func (s *Store) DeleteZone(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.zones[id]; !ok {
		return ErrNotFound
	}
	delete(s.zones, id)
	return s.persistZones()
}
