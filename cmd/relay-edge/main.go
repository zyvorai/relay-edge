// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/httpapi"
	"github.com/zyvorai/relay-edge/internal/logbuf"
	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
	"github.com/zyvorai/relay-edge/internal/tlsutil"
)

// Set via: go build -ldflags "-X main.version=v0.1.0"
var version = "dev"

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func envGatewayBase() string {
	if v, ok := os.LookupEnv("GATEWAY_BASE_URL"); ok {
		return v // empty string = direct Relay
	}
	return "https://127.0.0.1:8081"
}

func envBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func envInt64(k string, def int64) int64 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func splitSAN(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// envEnabledFamilies parses EDGE_ENABLED_FAMILIES (comma-separated). Unset
// or empty means "all families enabled" (nil slice — see httpapi.enabled).
// Recognized: farm, firewater, remote-edge, fleet.
func envEnabledFamilies() []string {
	v, ok := os.LookupEnv("EDGE_ENABLED_FAMILIES")
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	return splitSAN(v)
}

func main() {
	addr := env("EDGE_HTTP_ADDR", ":18086")
	dataDir := env("EDGE_DATA_DIR", "./data")
	tlsEnabled := envBool("EDGE_TLS", true)
	certPath := env("EDGE_TLS_CERT", filepath.Join(dataDir, "tls", "cert.pem"))
	keyPath := env("EDGE_TLS_KEY", filepath.Join(dataDir, "tls", "key.pem"))
	tlsSAN := env("EDGE_TLS_SAN", "localhost,127.0.0.1,relay-edge")

	_ = os.MkdirAll(dataDir, 0o755)

	logs := logbuf.New(500)
	out := logbuf.Multi(os.Stderr, logs)
	var logHandler slog.Handler
	if strings.EqualFold(env("EDGE_LOG_FORMAT", "text"), "json") {
		logHandler = slog.NewJSONHandler(out, nil)
	} else {
		logHandler = slog.NewTextHandler(out, nil)
	}
	slog.SetDefault(slog.New(logHandler))
	slog.Info("relay-edge starting", "version", version)

	seasons, err := season.Open(filepath.Join(dataDir, "seasons.json"))
	if err != nil {
		slog.Error("season store", "error", err)
		os.Exit(1)
	}
	sites, err := site.Open(filepath.Join(dataDir, "sites.json"), filepath.Join(dataDir, "zones.json"))
	if err != nil {
		slog.Error("site store", "error", err)
		os.Exit(1)
	}
	devices, err := device.Open(filepath.Join(dataDir, "devices.json"))
	if err != nil {
		slog.Error("device store", "error", err)
		os.Exit(1)
	}
	contacts, err := contact.Open(filepath.Join(dataDir, "contacts.json"))
	if err != nil {
		slog.Error("contact store", "error", err)
		os.Exit(1)
	}

	relayTLSInsecure := envBool("RELAY_TLS_INSECURE", true)
	if relayTLSInsecure && os.Getenv("RELAY_TLS_INSECURE") == "" {
		slog.Warn("RELAY_TLS_INSECURE defaulting to true — outbound TLS verification to Relay/gateway is disabled",
			"hint", "set RELAY_TLS_INSECURE=0 once Relay/gateway present a trusted certificate")
	}

	pub := &relaypub.Client{
		RelayBase:    env("RELAY_BASE_URL", "https://127.0.0.1:18080"),
		RelayToken:   env("RELAY_AUTH_TOKEN", ""),
		GatewayBase:  envGatewayBase(),
		GatewayToken: env("GATEWAY_AUTH_TOKEN", ""),
		Project:      envFirst("EDGE_GCP_PROJECT", "FASAL_GCP_PROJECT"),
		TLSInsecure:  relayTLSInsecure,
	}
	if pub.Project == "" {
		pub.Project = "fasal-onprem"
	}
	cfgPath := filepath.Join(dataDir, "runtime-config.json")
	if err := httpapi.LoadRuntimeConfig(cfgPath, pub); err != nil {
		slog.Warn("runtime-config", "error", err)
	}

	apiToken := strings.TrimSpace(os.Getenv("EDGE_API_TOKEN"))
	if envBool("EDGE_REQUIRE_AUTH", false) && apiToken == "" {
		slog.Error("EDGE_REQUIRE_AUTH=1 but EDGE_API_TOKEN is empty")
		os.Exit(1)
	}
	if apiToken == "" {
		slog.Warn("EDGE_API_TOKEN unset — API is open (lab mode)", "hint", "set EDGE_API_TOKEN for production")
	}

	api := httpapi.New(seasons, sites, devices, contacts, pub, envEnabledFamilies(), httpapi.Options{
		Version:      version,
		DataDir:      dataDir,
		ConfigPath:   cfgPath,
		TLSEnabled:   tlsEnabled,
		TLSCertPath:  certPath,
		APIToken:     apiToken,
		Logs:         logs,
		MaxBodyBytes: envInt64("EDGE_MAX_BODY_BYTES", 8<<20),

		// 0 (default) disables rate limiting entirely.
		RateLimitRPS:   float64(envInt64("EDGE_RATE_LIMIT_RPS", 0)),
		RateLimitBurst: int(envInt64("EDGE_RATE_LIMIT_BURST", 20)),
	})
	handler := api.Handler()

	scheme := "http"
	var srv *http.Server
	if tlsEnabled {
		scheme = "https"
		mat, err := tlsutil.LoadOrGenerateSelfSigned(certPath, keyPath, splitSAN(tlsSAN))
		if err != nil {
			slog.Error("tls", "error", err)
			os.Exit(1)
		}
		srv, err = tlsutil.NewServer(addr, mat, handler)
		if err != nil {
			slog.Error("tls server", "error", err)
			os.Exit(1)
		}
	} else {
		srv = &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       90 * time.Second,
		}
	}

	slog.Info("relay-edge listening", "version", version, "scheme", scheme, "addr", addr,
		"data_dir", dataDir, "gateway", pub.GatewayBase, "relay", pub.RelayBase,
		"tls", tlsEnabled, "auth_required", apiToken != "")

	errCh := make(chan error, 1)
	go func() {
		if tlsEnabled {
			errCh <- srv.ListenAndServeTLS("", "")
		} else {
			errCh <- srv.ListenAndServe()
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "error", err)
		}
	}
}
