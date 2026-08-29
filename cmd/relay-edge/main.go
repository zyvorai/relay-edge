// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	log.SetOutput(logbuf.Multi(os.Stderr, logs))
	logs.Append("relay-edge starting version=" + version)

	seasons, err := season.Open(filepath.Join(dataDir, "seasons.json"))
	if err != nil {
		log.Fatalf("season store: %v", err)
	}
	sites, err := site.Open(filepath.Join(dataDir, "sites.json"), filepath.Join(dataDir, "zones.json"))
	if err != nil {
		log.Fatalf("site store: %v", err)
	}
	devices, err := device.Open(filepath.Join(dataDir, "devices.json"))
	if err != nil {
		log.Fatalf("device store: %v", err)
	}
	contacts, err := contact.Open(filepath.Join(dataDir, "contacts.json"))
	if err != nil {
		log.Fatalf("contact store: %v", err)
	}

	pub := &relaypub.Client{
		RelayBase:    env("RELAY_BASE_URL", "https://127.0.0.1:18080"),
		RelayToken:   env("RELAY_AUTH_TOKEN", ""),
		GatewayBase:  envGatewayBase(),
		GatewayToken: env("GATEWAY_AUTH_TOKEN", ""),
		Project:      env("FASAL_GCP_PROJECT", "fasal-onprem"),
		TLSInsecure:  envBool("RELAY_TLS_INSECURE", true),
	}
	cfgPath := filepath.Join(dataDir, "runtime-config.json")
	if err := httpapi.LoadRuntimeConfig(cfgPath, pub); err != nil {
		log.Printf("runtime-config: %v", err)
	}

	api := httpapi.New(seasons, sites, devices, contacts, pub, envEnabledFamilies(), httpapi.Options{
		Version:     version,
		DataDir:     dataDir,
		ConfigPath:  cfgPath,
		TLSEnabled:  tlsEnabled,
		TLSCertPath: certPath,
		Logs:        logs,
	})
	handler := api.Handler()

	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	log.Printf("relay-edge %s listening on %s://%s (data=%s gateway=%s relay=%s tls=%v) © Zyvor AI Labs",
		version, scheme, addr, dataDir, pub.GatewayBase, pub.RelayBase, tlsEnabled)

	if tlsEnabled {
		mat, err := tlsutil.LoadOrGenerateSelfSigned(certPath, keyPath, splitSAN(tlsSAN))
		if err != nil {
			log.Fatalf("tls: %v", err)
		}
		if err := tlsutil.ListenAndServe(addr, mat, handler); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
		return
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
