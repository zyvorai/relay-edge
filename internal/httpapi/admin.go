// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zyvorai/relay-edge/internal/relaypub"
)

// RuntimeConfig is editable publish / lab wiring (persisted under data dir).
type RuntimeConfig struct {
	GatewayBaseURL   string `json:"gateway_base_url"`
	GatewayToken     string `json:"gateway_auth_token,omitempty"`
	RelayBaseURL     string `json:"relay_base_url"`
	RelayToken       string `json:"relay_auth_token,omitempty"`
	GCPProject       string `json:"fasal_gcp_project"`
	RelayTLSInsecure bool   `json:"relay_tls_insecure"`
}

type adminView struct {
	RuntimeConfig
	GatewayTokenSet bool     `json:"gateway_auth_token_set"`
	RelayTokenSet   bool     `json:"relay_auth_token_set"`
	Version         string   `json:"version"`
	Product         string   `json:"product"`
	Copyright       string   `json:"copyright"`
	VendorURL       string   `json:"vendor_url"`
	TLSEnabled      bool     `json:"tls_enabled"`
	TLSCert         string   `json:"tls_cert,omitempty"`
	DataDir         string   `json:"data_dir"`
	Modules         []string `json:"modules"`
	PublishPath     string   `json:"publish_path"`
}

func (s *Server) mountAdmin() {
	s.Mux.HandleFunc("GET /v1/admin/config", s.getAdminConfig)
	s.Mux.HandleFunc("PUT /v1/admin/config", s.putAdminConfig)
	s.Mux.HandleFunc("GET /v1/admin/logs", s.getAdminLogs)
	s.Mux.HandleFunc("POST /v1/admin/probe", s.postAdminProbe)
}

func (s *Server) getAdminConfig(w http.ResponseWriter, _ *http.Request) {
	s.pubMu.RLock()
	defer s.pubMu.RUnlock()
	writeJSON(w, 200, s.snapshotConfigLocked())
}

func (s *Server) snapshotConfigLocked() adminView {
	view := adminView{
		Version:   s.version,
		Product:   "relay-edge",
		Copyright: "© 2026 Zyvor AI Labs",
		VendorURL: "https://zyvor.dev",
		TLSEnabled: s.tlsEnabled,
		TLSCert:    s.tlsCertPath,
		DataDir:    s.dataDir,
		Modules:     append([]string{}, s.mountedModules...),
		PublishPath: s.publishPathLocked(),
	}
	if s.Pub != nil {
		view.GatewayBaseURL = s.Pub.GatewayBase
		view.RelayBaseURL = s.Pub.RelayBase
		view.GCPProject = s.Pub.Project
		view.RelayTLSInsecure = s.Pub.TLSInsecure
		view.GatewayTokenSet = s.Pub.GatewayToken != ""
		view.RelayTokenSet = s.Pub.RelayToken != ""
	}
	return view
}

func (s *Server) putAdminConfig(w http.ResponseWriter, r *http.Request) {
	var in RuntimeConfig
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	if s.Pub == nil {
		writeJSON(w, 500, map[string]any{"error": "publisher unavailable"})
		return
	}
	applyRuntime(s.Pub, in, false)
	if err := s.persistConfigLocked(); err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if s.Logs != nil {
		s.Logs.Append("admin: runtime config updated path=" + s.publishPathLocked())
	}
	writeJSON(w, 200, s.snapshotConfigLocked())
}

func applyRuntime(pub *relaypub.Client, in RuntimeConfig, onlyIfEmpty bool) {
	if pub == nil {
		return
	}
	if !onlyIfEmpty || pub.GatewayBase == "" {
		// Always allow clearing gateway for direct mode when not onlyIfEmpty
		if !onlyIfEmpty {
			pub.GatewayBase = strings.TrimSpace(in.GatewayBaseURL)
		} else if strings.TrimSpace(in.GatewayBaseURL) != "" {
			pub.GatewayBase = strings.TrimSpace(in.GatewayBaseURL)
		}
	}
	if v := strings.TrimSpace(in.RelayBaseURL); v != "" && (!onlyIfEmpty || pub.RelayBase == "") {
		pub.RelayBase = v
	}
	if v := strings.TrimSpace(in.GCPProject); v != "" && (!onlyIfEmpty || pub.Project == "") {
		pub.Project = v
	}
	if !onlyIfEmpty {
		pub.TLSInsecure = in.RelayTLSInsecure
	}
	if in.GatewayToken != "" {
		pub.GatewayToken = in.GatewayToken
	}
	if in.RelayToken != "" {
		pub.RelayToken = in.RelayToken
	}
	pub.HTTP = nil
}

func (s *Server) persistConfigLocked() error {
	if s.configPath == "" || s.Pub == nil {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(s.configPath), 0o755)
	cfg := RuntimeConfig{
		GatewayBaseURL:   s.Pub.GatewayBase,
		GatewayToken:     s.Pub.GatewayToken,
		RelayBaseURL:     s.Pub.RelayBase,
		RelayToken:       s.Pub.RelayToken,
		GCPProject:       s.Pub.Project,
		RelayTLSInsecure: s.Pub.TLSInsecure,
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, b, 0o600)
}

// LoadRuntimeConfig overlays saved config onto an existing publisher client.
func LoadRuntimeConfig(path string, pub *relaypub.Client) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var cfg RuntimeConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}
	applyRuntime(pub, cfg, false)
	return nil
}

func (s *Server) getAdminLogs(w http.ResponseWriter, r *http.Request) {
	n := 200
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	var lines []string
	if s.Logs != nil {
		lines = s.Logs.Lines(n)
	}
	writeJSON(w, 200, map[string]any{"items": lines, "n": len(lines)})
}

func (s *Server) postAdminProbe(w http.ResponseWriter, _ *http.Request) {
	s.pubMu.RLock()
	pub := s.Pub
	s.pubMu.RUnlock()
	out := map[string]any{
		"edge":    map[string]any{"ok": true, "path": s.publishPath()},
		"gateway": map[string]any{"ok": false},
		"relay":   map[string]any{"ok": false},
	}
	if pub == nil {
		writeJSON(w, 200, out)
		return
	}
	client := pub.HTTPClient()
	gw := out["gateway"].(map[string]any)
	rl := out["relay"].(map[string]any)
	if pub.GatewayBase != "" {
		u := strings.TrimRight(pub.GatewayBase, "/") + "/healthz"
		gw["url"] = u
		resp, err := client.Get(u)
		if err != nil {
			gw["error"] = err.Error()
		} else {
			_ = resp.Body.Close()
			gw["ok"] = resp.StatusCode/100 == 2
			gw["status"] = resp.StatusCode
		}
	} else {
		gw["ok"] = true
		gw["skipped"] = "direct mode"
	}
	if pub.RelayBase != "" {
		u := strings.TrimRight(pub.RelayBase, "/") + "/healthz"
		rl["url"] = u
		resp, err := client.Get(u)
		if err != nil {
			rl["error"] = err.Error()
		} else {
			_ = resp.Body.Close()
			rl["ok"] = resp.StatusCode/100 == 2
			rl["status"] = resp.StatusCode
		}
	}
	writeJSON(w, 200, out)
}
