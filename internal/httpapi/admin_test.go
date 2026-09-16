// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/httpapi"
	"github.com/zyvorai/relay-edge/internal/relaypub"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
)

func newAdminTestServer(t *testing.T, token string) (*httpapi.Server, string) {
	t.Helper()
	dir := t.TempDir()
	seasons, _ := season.Open(filepath.Join(dir, "seasons.json"))
	sites, _ := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	devices, _ := device.Open(filepath.Join(dir, "devices.json"))
	contacts, _ := contact.Open(filepath.Join(dir, "contacts.json"))
	cfgPath := filepath.Join(dir, "runtime-config.json")
	s := httpapi.New(seasons, sites, devices, contacts,
		&relaypub.Client{},
		nil,
		httpapi.Options{Version: "test", APIToken: token, ConfigPath: cfgPath},
	)
	return s, cfgPath
}

func TestPutAdminConfig_MalformedJSON(t *testing.T) {
	s, _ := newAdminTestServer(t, "")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/config", strings.NewReader(`{"gateway_base_url":`))
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed JSON, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPutAdminConfig_WrongToken(t *testing.T) {
	s, _ := newAdminTestServer(t, "secret")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/config", strings.NewReader(`{"gateway_base_url":"https://gw"}`))
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for wrong token, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPutAdminConfig_UpdatesAndPersists(t *testing.T) {
	s, cfgPath := newAdminTestServer(t, "")
	body := `{"gateway_base_url":"https://gw.example","gateway_auth_token":"super-secret-token"}`
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/v1/admin/config", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/admin/config", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"gateway_base_url":"https://gw.example"`) {
		t.Fatalf("config not updated: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"gateway_auth_token_set":true`) {
		t.Fatalf("gateway_auth_token_set not reflected: %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "super-secret-token") {
		t.Fatalf("token leaked in admin config response: %s", rr.Body.String())
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "super-secret-token") {
		t.Fatalf("token leaked to disk: %s", b)
	}
}

func TestGetAdminLogs_InvalidNFallsBackToDefault(t *testing.T) {
	s, _ := newAdminTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/admin/logs?n=not-a-number", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 despite bad ?n=, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPostAdminProbe_NoGatewayConfigured(t *testing.T) {
	s, _ := newAdminTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/admin/probe", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"skipped":"direct mode"`) {
		t.Fatalf("want gateway probe skipped in direct mode: %s", rr.Body.String())
	}
}

func TestGetGatewayInventory_NotConfigured(t *testing.T) {
	s, _ := newAdminTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/admin/gateway/inventory", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 when gateway_base_url unset, got %d %s", rr.Code, rr.Body.String())
	}
}
