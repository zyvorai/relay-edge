// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zyvorai/relay-edge/internal/site"
)

func (s *Server) listSites(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.Sites.ListSites()})
}

func (s *Server) getSite(w http.ResponseWriter, r *http.Request) {
	it, err := s.Sites.GetSite(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, 200, it)
}

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var in site.Site
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if in.ID == "" {
		in.ID = newID("site")
	}
	out, err := s.Sites.PutSite(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, out)
}

func (s *Server) putSite(w http.ResponseWriter, r *http.Request) {
	var in site.Site
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	in.ID = r.PathValue("id")
	out, err := s.Sites.PutSite(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) deleteSite(w http.ResponseWriter, r *http.Request) {
	if err := s.Sites.DeleteSite(r.PathValue("id")); err != nil {
		if errors.Is(err, site.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) putSiteRouting(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Routing map[string]string `json:"routing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	out, err := s.Sites.SetRouting(r.PathValue("id"), body.Routing)
	if err != nil {
		if errors.Is(err, site.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) listZones(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.Sites.ListZones(r.PathValue("id"))})
}

func (s *Server) listAllZones(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	writeJSON(w, 200, map[string]any{"items": s.Sites.ListZones(siteID)})
}

func (s *Server) createZone(w http.ResponseWriter, r *http.Request) {
	var in site.Zone
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	in.SiteID = r.PathValue("id")
	if in.ID == "" {
		in.ID = newID("zone")
	}
	out, err := s.Sites.PutZone(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, out)
}

func (s *Server) getZone(w http.ResponseWriter, r *http.Request) {
	it, err := s.Sites.GetZone(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, 200, it)
}

func (s *Server) putZone(w http.ResponseWriter, r *http.Request) {
	var in site.Zone
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	in.ID = r.PathValue("id")
	if in.SiteID == "" {
		if prev, err := s.Sites.GetZone(in.ID); err == nil {
			in.SiteID = prev.SiteID
		}
	}
	out, err := s.Sites.PutZone(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) deleteZone(w http.ResponseWriter, r *http.Request) {
	if err := s.Sites.DeleteZone(r.PathValue("id")); err != nil {
		if errors.Is(err, site.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) getTelemetry(w http.ResponseWriter, r *http.Request) {
	it, err := s.Sites.GetZone(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	if it.Telemetry == nil {
		writeJSON(w, 200, map[string]any{"telemetry": nil})
		return
	}
	writeJSON(w, 200, map[string]any{"telemetry": it.Telemetry})
}

func (s *Server) putTelemetry(w http.ResponseWriter, r *http.Request) {
	var probe site.VerificationProbe
	if err := json.NewDecoder(r.Body).Decode(&probe); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if probe.URL == "" {
		writeJSON(w, 400, map[string]any{"error": "url required"})
		return
	}
	out, err := s.Sites.SetTelemetry(r.PathValue("id"), &probe)
	if err != nil {
		if errors.Is(err, site.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) deleteTelemetry(w http.ResponseWriter, r *http.Request) {
	out, err := s.Sites.SetTelemetry(r.PathValue("id"), nil)
	if err != nil {
		if errors.Is(err, site.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}
