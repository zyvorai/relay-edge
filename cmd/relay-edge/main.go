// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/httpapi"
	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	addr := env("EDGE_HTTP_ADDR", ":18086")
	dataDir := env("EDGE_DATA_DIR", "./data")
	_ = os.MkdirAll(dataDir, 0o755)

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
		GatewayBase:  env("GATEWAY_BASE_URL", "http://127.0.0.1:18083"),
		GatewayToken: env("GATEWAY_AUTH_TOKEN", ""),
		Project:      env("FASAL_GCP_PROJECT", "fasal-onprem"),
		TLSInsecure:  env("RELAY_TLS_INSECURE", "1") == "1",
	}

	api := httpapi.New(seasons, sites, devices, contacts, pub)
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	log.Printf("relay-edge listening on %s (data=%s gateway=%s relay=%s)", addr, dataDir, pub.GatewayBase, pub.RelayBase)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
