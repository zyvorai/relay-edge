// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
)

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.Devices.List(r.URL.Query().Get("zone_id"))})
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	it, err := s.Devices.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, 200, it)
}

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	var in device.Device
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if in.ID == "" {
		in.ID = newID("dev")
	}
	if _, err := s.Sites.GetZone(in.ZoneID); err != nil {
		writeJSON(w, 400, map[string]any{"error": "zone_id not found"})
		return
	}
	out, err := s.Devices.Put(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, out)
}

func (s *Server) putDevice(w http.ResponseWriter, r *http.Request) {
	var in device.Device
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	in.ID = r.PathValue("id")
	out, err := s.Devices.Put(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	if err := s.Devices.Delete(r.PathValue("id")); err != nil {
		if errors.Is(err, device.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) listContacts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.Contacts.List()})
}

func (s *Server) getContact(w http.ResponseWriter, r *http.Request) {
	it, err := s.Contacts.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, 200, it)
}

func (s *Server) createContact(w http.ResponseWriter, r *http.Request) {
	var in contact.Contact
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if in.ID == "" {
		in.ID = newID("contact")
	}
	out, err := s.Contacts.Put(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, out)
}

func (s *Server) putContact(w http.ResponseWriter, r *http.Request) {
	var in contact.Contact
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	in.ID = r.PathValue("id")
	out, err := s.Contacts.Put(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) deleteContact(w http.ResponseWriter, r *http.Request) {
	if err := s.Contacts.Delete(r.PathValue("id")); err != nil {
		if errors.Is(err, contact.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}
