// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/zyvorai/relay-edge/internal/httpapi"
	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
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
	storePath := dataDir + "/seasons.json"

	st, err := season.Open(storePath)
	if err != nil {
		log.Fatalf("season store: %v", err)
	}

	pub := &relaypub.Client{
		RelayBase:   env("RELAY_BASE_URL", "https://127.0.0.1:18080"),
		RelayToken:  env("RELAY_AUTH_TOKEN", ""),
		GatewayBase: env("GATEWAY_BASE_URL", "http://127.0.0.1:18083"),
		Project:     env("FASAL_GCP_PROJECT", "fasal-onprem"),
		TLSInsecure: env("RELAY_TLS_INSECURE", "1") == "1",
	}

	api := httpapi.New(st, pub)
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	log.Printf("relay-edge listening on %s (store=%s gateway=%s relay=%s)", addr, storePath, pub.GatewayBase, pub.RelayBase)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
