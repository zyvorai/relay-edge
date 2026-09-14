package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zyvorai/relay-edge/internal/jsonstore"
	"github.com/zyvorai/relay-edge/internal/relaypub"
)

func TestPersistConfigOmitsTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime-config.json")
	s := &Server{
		configPath: path,
		Pub: &relaypub.Client{
			GatewayBase:  "https://gw.example",
			RelayBase:    "https://relay.example",
			Project:      "proj",
			TLSInsecure:  true,
			GatewayToken: "secret-gw",
			RelayToken:   "secret-relay",
		},
	}
	if err := s.persistConfigLocked(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret-") {
		t.Fatalf("tokens leaked to disk: %s", b)
	}
	var cfg RuntimeConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.GatewayBaseURL != "https://gw.example" || cfg.RelayToken != "" || cfg.GatewayToken != "" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestLoadRuntimeConfigStripsLegacyTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime-config.json")
	legacy := []byte(`{"gateway_base_url":"https://gw","relay_base_url":"https://r","gateway_auth_token":"old-gw","relay_auth_token":"old-r"}`)
	if err := jsonstore.WriteAtomic(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	pub := &relaypub.Client{GatewayToken: "keep-env", RelayToken: "keep-env"}
	if err := LoadRuntimeConfig(path, pub); err != nil {
		t.Fatal(err)
	}
	if pub.GatewayBase != "https://gw" || pub.RelayBase != "https://r" {
		t.Fatalf("urls not applied: %+v", pub)
	}
	if pub.GatewayToken != "keep-env" || pub.RelayToken != "keep-env" {
		t.Fatalf("legacy tokens must not overwrite env tokens: %+v", pub)
	}
}
