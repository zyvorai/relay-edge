// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sync"

	"github.com/zyvorai/relay-edge/internal/remoteedge"
	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/firewater"
	"github.com/zyvorai/relay-edge/internal/fleet"
	"github.com/zyvorai/relay-edge/internal/logbuf"
	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
)

type Server struct {
	Seasons  *season.Store
	Sites    *site.Store
	Devices  *device.Store
	Contacts *contact.Store
	Pub      *relaypub.Client
	Mux      *http.ServeMux

	FW            *firewater.Engine
	RemoteEdge    *remoteedge.Engine
	Fleet         *fleet.Engine
	remoteEdgePublish  bool
	remoteEdgeInterval int
	fleetPublish  bool
	fleetInterval int
	fwMu          sync.Mutex
	fwSubs        map[chan []byte]struct{}
	fwEventsLog   []firewater.Event
	remoteEdgeMu       sync.Mutex
	remoteEdgeSubs     map[chan []byte]struct{}
	remoteEdgeEventsLog []remoteedge.Event
	fleetMu       sync.Mutex
	fleetSubs     map[chan []byte]struct{}
	fleetEventsLog []fleet.Event

	mountedModules []string
	version        string

	pubMu       sync.RWMutex
	Logs        *logbuf.Ring
	dataDir     string
	configPath  string
	tlsEnabled  bool
	tlsCertPath string
	apiToken    string
	metrics     *metrics
}

// Options configures optional server wiring (TLS metadata, logs, config path).
type Options struct {
	Version     string
	DataDir     string
	ConfigPath  string
	TLSEnabled  bool
	TLSCertPath string
	APIToken    string
	Logs        *logbuf.Ring
}

// enabled reports whether family is present in families (case-sensitive,
// exact match). A nil/empty families means "everything enabled" — the
// default when EDGE_ENABLED_FAMILIES is unset.
func enabled(families []string, family string) bool {
	if len(families) == 0 {
		return true
	}
	for _, f := range families {
		if f == family {
			return true
		}
	}
	return false
}

// New builds the relay-edge HTTP server. enabledFamilies restricts which of
// firewater/remote-edge/fleet get mounted (nil/empty = all); the farm-ish
// routes (seasons/sites/zones/devices/contacts) mount unconditionally since
// they have no equivalent gate today. version is reported on /healthz and /readyz.
func New(seasons *season.Store, sites *site.Store, devices *device.Store, contacts *contact.Store, pub *relaypub.Client, enabledFamilies []string, opts Options) *Server {
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	s := &Server{
		Seasons: seasons, Sites: sites, Devices: devices, Contacts: contacts, Pub: pub,
		Mux: http.NewServeMux(), version: version,
		Logs: opts.Logs, dataDir: opts.DataDir, configPath: opts.ConfigPath,
		tlsEnabled: opts.TLSEnabled, tlsCertPath: opts.TLSCertPath,
		apiToken: opts.APIToken,
		metrics:  &metrics{started: time.Now()},
	}
	s.mountedModules = []string{"seasons", "sites", "zones", "devices", "contacts", "telemetry", "stages"}
	s.Mux.HandleFunc("GET /healthz", s.health)
	s.Mux.HandleFunc("GET /readyz", s.ready)
	s.Mux.HandleFunc("GET /version", s.versionHandler)
	s.Mux.HandleFunc("GET /metrics", s.metricsHandler)
	s.mountAdmin()

	s.Mux.HandleFunc("GET /v1/sites", s.listSites)
	s.Mux.HandleFunc("POST /v1/sites", s.createSite)
	s.Mux.HandleFunc("GET /v1/sites/{id}", s.getSite)
	s.Mux.HandleFunc("PUT /v1/sites/{id}", s.putSite)
	s.Mux.HandleFunc("DELETE /v1/sites/{id}", s.deleteSite)
	s.Mux.HandleFunc("PUT /v1/sites/{id}/routing", s.putSiteRouting)
	s.Mux.HandleFunc("GET /v1/sites/{id}/zones", s.listZones)
	s.Mux.HandleFunc("POST /v1/sites/{id}/zones", s.createZone)

	s.Mux.HandleFunc("GET /v1/zones", s.listAllZones)
	s.Mux.HandleFunc("GET /v1/zones/{id}", s.getZone)
	s.Mux.HandleFunc("PUT /v1/zones/{id}", s.putZone)
	s.Mux.HandleFunc("DELETE /v1/zones/{id}", s.deleteZone)
	s.Mux.HandleFunc("GET /v1/zones/{id}/telemetry", s.getTelemetry)
	s.Mux.HandleFunc("PUT /v1/zones/{id}/telemetry", s.putTelemetry)
	s.Mux.HandleFunc("DELETE /v1/zones/{id}/telemetry", s.deleteTelemetry)

	s.Mux.HandleFunc("GET /v1/devices", s.listDevices)
	s.Mux.HandleFunc("POST /v1/devices", s.createDevice)
	s.Mux.HandleFunc("GET /v1/devices/{id}", s.getDevice)
	s.Mux.HandleFunc("PUT /v1/devices/{id}", s.putDevice)
	s.Mux.HandleFunc("DELETE /v1/devices/{id}", s.deleteDevice)

	s.Mux.HandleFunc("GET /v1/contacts", s.listContacts)
	s.Mux.HandleFunc("POST /v1/contacts", s.createContact)
	s.Mux.HandleFunc("GET /v1/contacts/{id}", s.getContact)
	s.Mux.HandleFunc("PUT /v1/contacts/{id}", s.putContact)
	s.Mux.HandleFunc("DELETE /v1/contacts/{id}", s.deleteContact)

	s.Mux.HandleFunc("GET /v1/seasons", s.listSeasons)
	s.Mux.HandleFunc("POST /v1/seasons", s.createSeason)
	s.Mux.HandleFunc("GET /v1/seasons/{id}", s.getSeason)
	s.Mux.HandleFunc("PUT /v1/seasons/{id}", s.putSeason)
	s.Mux.HandleFunc("DELETE /v1/seasons/{id}", s.deleteSeason)
	s.Mux.HandleFunc("POST /v1/seasons/{id}/open", s.openSeason)
	s.Mux.HandleFunc("POST /v1/seasons/{id}/close", s.closeSeason)
	s.Mux.HandleFunc("POST /v1/seasons/{id}/events", s.publishSeasonEvent)
	s.Mux.HandleFunc("POST /v1/seasons/{id}/stage", s.setSeasonStage)
	s.Mux.HandleFunc("POST /v1/seasons/{id}/advisories", s.publishAdvisory)
	if enabled(enabledFamilies, "firewater") {
		s.mountFirewater()
		s.mountedModules = append(s.mountedModules, "firewater")
	}
	if enabled(enabledFamilies, "remote-edge") {
		s.mountRemoteEdge()
		s.mountedModules = append(s.mountedModules, "remote-edge")
	}
	if enabled(enabledFamilies, "fleet") {
		s.mountFleet()
		s.mountedModules = append(s.mountedModules, "fleet")
	}
	s.mountUI()
	s.mountedModules = append(s.mountedModules, "ui")
	return s
}

func (s *Server) Handler() http.Handler {
	return s.withAuth(s.withMetrics(s.Mux))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":        "ok",
		"product":       "relay-edge",
		"version":       s.version,
		"modules":       s.mountedModules,
		"copyright":     "© 2026 Zyvor AI Labs",
		"vendor":        "https://zyvor.dev",
		"tls":           s.tlsEnabled,
		"auth_required": s.apiToken != "",
		"time":          time.Now().UTC(),
	})
}

func (s *Server) versionHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"product": "relay-edge",
		"version": s.version,
	})
}

// ready is stricter than healthz: stores must be present and a publish target
// (gateway or direct Relay base) must be configured.
func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	if s.Seasons == nil || s.Sites == nil || s.Devices == nil || s.Contacts == nil {
		writeJSON(w, 503, map[string]any{"status": "not_ready", "reason": "stores unavailable", "version": s.version})
		return
	}
	s.pubMu.RLock()
	pub := s.Pub
	s.pubMu.RUnlock()
	if pub == nil || (strings.TrimSpace(pub.GatewayBase) == "" && strings.TrimSpace(pub.RelayBase) == "") {
		writeJSON(w, 503, map[string]any{"status": "not_ready", "reason": "no publish target", "version": s.version})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "relay-edge",
		"version": s.version,
		"modules": s.mountedModules,
		"path":    s.publishPath(),
		"time":    time.Now().UTC(),
	})
}

func (s *Server) publishPath() string {
	s.pubMu.RLock()
	defer s.pubMu.RUnlock()
	return s.publishPathLocked()
}

func (s *Server) publishPathLocked() string {
	if s.Pub != nil && strings.TrimSpace(s.Pub.GatewayBase) != "" {
		return "gateway"
	}
	return "relay"
}

// --- enrichment ---

type enrichCtx struct {
	Season  season.Season
	Site    *site.Site
	Zone    *site.Zone
	Device  *device.Device
	Contact *contact.Contact
}

func (s *Server) resolveEnrich(it season.Season, zoneID, zoneCode, deviceID string) enrichCtx {
	ctx := enrichCtx{Season: it}
	if it.SiteID != "" {
		if sit, err := s.Sites.GetSite(it.SiteID); err == nil {
			ctx.Site = &sit
			// Prefer farmer → operator → any routing contact
			for _, role := range []string{"farmer", "operator", "agronomist", "ehs", "control_room"} {
				if cid := sit.Routing[role]; cid != "" {
					if c, err := s.Contacts.Get(cid); err == nil {
						ctx.Contact = &c
						break
					}
				}
			}
		}
	}
	if zoneID != "" {
		if z, err := s.Sites.GetZone(zoneID); err == nil {
			ctx.Zone = &z
		}
	} else if zoneCode != "" && it.SiteID != "" {
		for _, z := range s.Sites.ListZones(it.SiteID) {
			if z.Code == zoneCode || z.Name == zoneCode {
				zz := z
				ctx.Zone = &zz
				break
			}
		}
	}
	if deviceID != "" {
		if d, err := s.Devices.Get(deviceID); err == nil {
			ctx.Device = &d
		}
	} else if ctx.Zone != nil {
		if d, ok := s.Devices.FirstForZone(ctx.Zone.ID); ok {
			ctx.Device = &d
		}
	}
	return ctx
}

func (s *Server) stampData(ctx enrichCtx, extra map[string]any) map[string]any {
	it := ctx.Season
	out := map[string]any{
		"season_id":   it.ID,
		"season_name": it.Name,
		"crop":        it.Crop,
		"site":        it.Site,
		"stage":       it.Stage,
	}
	if it.SiteID != "" {
		out["site_id"] = it.SiteID
	}
	if ctx.Site != nil {
		out["site"] = ctx.Site.Name
		out["site_id"] = ctx.Site.ID
	}
	if ctx.Zone != nil {
		out["zone_id"] = ctx.Zone.ID
		out["zone"] = ctx.Zone.Code
	}
	if ctx.Device != nil {
		out["device_id"] = ctx.Device.ID
		out["fasal_device_id"] = ctx.Device.ExternalID
	}
	if ctx.Contact != nil {
		if ctx.Contact.FCMToken != "" {
			out["recipient"] = ctx.Contact.FCMToken
		}
		if ctx.Contact.SMS != "" {
			out["sms_recipient"] = ctx.Contact.SMS
		}
		if ctx.Contact.Email != "" {
			out["email_recipient"] = ctx.Contact.Email
		}
	}
	if _, ok := out["recipient"]; !ok {
		out["recipient"] = "demo-device-token"
	}
	for k, v := range it.Labels {
		out["label_"+k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	// Inject verification_probe for critical paths when zone has telemetry and caller did not set one.
	if ctx.Zone != nil && ctx.Zone.Telemetry != nil && ctx.Zone.Telemetry.URL != "" {
		if _, exists := out["verification_probe"]; !exists {
			probe := map[string]any{"url": ctx.Zone.Telemetry.URL}
			if ctx.Zone.Telemetry.Method != "" {
				probe["method"] = ctx.Zone.Telemetry.Method
			}
			if ctx.Zone.Telemetry.JSONPath != "" {
				probe["json_path"] = ctx.Zone.Telemetry.JSONPath
			}
			if ctx.Zone.Telemetry.Expect != "" {
				probe["expect"] = ctx.Zone.Telemetry.Expect
			}
			out["verification_probe"] = probe
		}
	}
	return out
}

func seasonSource(ctx enrichCtx) string {
	parts := []string{}
	if ctx.Site != nil {
		parts = append(parts, ctx.Site.Name)
	} else if ctx.Season.Site != "" {
		parts = append(parts, ctx.Season.Site)
	}
	parts = append(parts, ctx.Season.Name)
	if ctx.Season.Crop != "" {
		parts = append(parts, ctx.Season.Crop)
	}
	if ctx.Zone != nil {
		parts = append(parts, "Zone "+ctx.Zone.Code)
	}
	return strings.Join(parts, " / ")
}

func (s *Server) resolveSeasonSite(in *season.Season) error {
	if in.SiteID != "" {
		sit, err := s.Sites.GetSite(in.SiteID)
		if err != nil {
			return errors.New("site_id not found")
		}
		if in.Site == "" {
			in.Site = sit.Name
		}
		return nil
	}
	return nil
}

// --- seasons ---

func (s *Server) listSeasons(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.Seasons.List()})
}

func (s *Server) getSeason(w http.ResponseWriter, r *http.Request) {
	it, err := s.Seasons.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, 200, it)
}

func (s *Server) createSeason(w http.ResponseWriter, r *http.Request) {
	var in season.Season
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if in.ID == "" {
		in.ID = newID("season")
	}
	if err := s.resolveSeasonSite(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	out, err := s.Seasons.Put(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, out)
}

func (s *Server) putSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in season.Season
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	in.ID = id
	if err := s.resolveSeasonSite(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	out, err := s.Seasons.Put(in)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) deleteSeason(w http.ResponseWriter, r *http.Request) {
	if err := s.Seasons.Delete(r.PathValue("id")); err != nil {
		if errors.Is(err, season.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) openSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, err := s.Seasons.Get(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	if it.Status == "active" {
		writeJSON(w, 200, map[string]any{"season": it, "note": "already active"})
		return
	}
	ctx := s.resolveEnrich(it, "", "", "")
	key := fmt.Sprintf("edge/season/%s/open/%d", id, time.Now().Unix())
	data := s.stampData(ctx, map[string]any{
		"advisory": fmt.Sprintf("Season %s opened for crop %s at site %s", it.Name, it.Crop, siteName(ctx)),
	})
	s.IncPublish()
	res, err := s.Pub.PublishEventType("crop.advisory", "info", seasonSource(ctx), key, data)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	updated, _ := s.Seasons.UpdateStatus(id, "active", res.EventID)
	writeJSON(w, 200, map[string]any{"season": updated, "publish": res})
}

func (s *Server) closeSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, err := s.Seasons.Get(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	ctx := s.resolveEnrich(it, "", "", "")
	key := fmt.Sprintf("edge/season/%s/close/%d", id, time.Now().Unix())
	data := s.stampData(ctx, map[string]any{
		"advisory": fmt.Sprintf("Season %s closed", it.Name),
	})
	s.IncPublish()
	res, err := s.Pub.PublishEventType("crop.advisory", "info", seasonSource(ctx), key, data)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	updated, _ := s.Seasons.UpdateStatus(id, "closed", res.EventID)
	writeJSON(w, 200, map[string]any{"season": updated, "publish": res})
}

type seasonEventIn struct {
	Type           string         `json:"type"`
	Severity       string         `json:"severity"`
	IdempotencyKey string         `json:"idempotency_key"`
	Data           map[string]any `json:"data"`
	Command        string         `json:"command,omitempty"`
	Zone           string         `json:"zone,omitempty"`
	ZoneID         string         `json:"zone_id,omitempty"`
	DeviceID       string         `json:"device_id,omitempty"`
}

func (s *Server) publishSeasonEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, err := s.Seasons.Get(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	if it.Status != "active" {
		writeJSON(w, 409, map[string]any{"error": "season must be active to publish critical farm events", "status": it.Status})
		return
	}
	var in seasonEventIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if in.Type == "" {
		writeJSON(w, 400, map[string]any{"error": "type required"})
		return
	}
	if in.Severity == "" {
		in.Severity = "info"
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = fmt.Sprintf("edge/season/%s/%s/%d", id, in.Type, time.Now().UnixNano())
	}
	extra := in.Data
	if extra == nil {
		extra = map[string]any{}
	}
	ctx := s.resolveEnrich(it, in.ZoneID, in.Zone, in.DeviceID)
	if in.Command != "" {
		zoneCode := in.Zone
		if zoneCode == "" && ctx.Zone != nil {
			zoneCode = ctx.Zone.Code
		}
		if zoneCode == "" {
			zoneCode = "A4"
		}
		payload := map[string]any{"zone": zoneCode, "season_id": it.ID}
		if ctx.Zone != nil {
			payload["zone_id"] = ctx.Zone.ID
		}
		if ctx.Device != nil {
			payload["fasal_device_id"] = ctx.Device.ExternalID
			payload["device_id"] = ctx.Device.ID
		}
		if dur, ok := extra["duration_minutes"]; ok {
			payload["duration_minutes"] = dur
		}
		extra["recommended_action"] = map[string]any{
			"target":  "farm-controller",
			"command": in.Command,
			"payload": payload,
		}
		if in.Severity == "info" {
			in.Severity = "critical"
		}
	}
	data := s.stampData(ctx, extra)
	s.IncPublish()
	res, err := s.Pub.PublishEventType(in.Type, in.Severity, seasonSource(ctx), in.IdempotencyKey, data)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if res.EventID != "" {
		_, _ = s.Seasons.UpdateStatus(it.ID, it.Status, res.EventID)
	}
	writeJSON(w, 200, map[string]any{"season_id": it.ID, "publish": res, "stamped": map[string]any{
		"site_id": data["site_id"], "zone_id": data["zone_id"], "zone": data["zone"],
		"device_id": data["device_id"], "fasal_device_id": data["fasal_device_id"],
	}})
}

type stageIn struct {
	Stage string `json:"stage"`
}

func (s *Server) setSeasonStage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in stageIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	updated, err := s.Seasons.UpdateStage(id, in.Stage)
	if err != nil {
		if errors.Is(err, season.ErrNotFound) {
			writeJSON(w, 404, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	ctx := s.resolveEnrich(updated, "", "", "")
	key := fmt.Sprintf("edge/season/%s/stage/%s/%d", id, in.Stage, time.Now().Unix())
	data := s.stampData(ctx, map[string]any{
		"advisory": fmt.Sprintf("Season %s entered growth stage %s", updated.Name, in.Stage),
		"stage":    in.Stage,
	})
	s.IncPublish()
	res, err := s.Pub.PublishEventType("crop.advisory", "info", seasonSource(ctx), key, data)
	if err != nil {
		writeJSON(w, 200, map[string]any{"season": updated, "publish_error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"season": updated, "publish": res})
}

type advisoryIn struct {
	Type           string         `json:"type"` // crop.advisory|spray.advisory|frost.alert|weather.advisory|pest.advisory
	Severity       string         `json:"severity"`
	Message        string         `json:"message"`
	IdempotencyKey string         `json:"idempotency_key"`
	Data           map[string]any `json:"data"`
}

func (s *Server) publishAdvisory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, err := s.Seasons.Get(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	var in advisoryIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if in.Type == "" {
		in.Type = "crop.advisory"
	}
	if in.Severity == "" {
		in.Severity = "info"
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = fmt.Sprintf("edge/season/%s/advisory/%s/%d", id, in.Type, time.Now().UnixNano())
	}
	extra := in.Data
	if extra == nil {
		extra = map[string]any{}
	}
	if in.Message != "" {
		extra["advisory"] = in.Message
	}
	ctx := s.resolveEnrich(it, "", "", "")
	data := s.stampData(ctx, extra)
	s.IncPublish()
	res, err := s.Pub.PublishEventType(in.Type, in.Severity, seasonSource(ctx), in.IdempotencyKey, data)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"season_id": it.ID, "publish": res})
}

func siteName(ctx enrichCtx) string {
	if ctx.Site != nil {
		return ctx.Site.Name
	}
	return ctx.Season.Site
}
