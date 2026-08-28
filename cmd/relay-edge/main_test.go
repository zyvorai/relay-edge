// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"
)

func TestEnvGatewayBase(t *testing.T) {
	t.Setenv("GATEWAY_BASE_URL", "")
	if got := envGatewayBase(); got != "" {
		t.Fatalf("empty GATEWAY_BASE_URL: got %q want direct (empty)", got)
	}

	os.Unsetenv("GATEWAY_BASE_URL")
	if got := envGatewayBase(); got != "https://127.0.0.1:8081" {
		t.Fatalf("unset GATEWAY_BASE_URL: got %q want default gateway", got)
	}

	t.Setenv("GATEWAY_BASE_URL", "https://gw.example:8081")
	if got := envGatewayBase(); got != "https://gw.example:8081" {
		t.Fatalf("custom gateway: got %q", got)
	}
}
