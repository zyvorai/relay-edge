// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"net/http/httptest"
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

func newSitesTestServer(t *testing.T, token string) *httpapi.Server {
	t.Helper()
	dir := t.TempDir()
	seasons, _ := season.Open(filepath.Join(dir, "seasons.json"))
	sites, _ := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	devices, _ := device.Open(filepath.Join(dir, "devices.json"))
	contacts, _ := contact.Open(filepath.Join(dir, "contacts.json"))
	if _, err := sites.PutSite(site.Site{ID: "site1", Name: "Site 1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := sites.PutZone(site.Zone{ID: "z1", SiteID: "site1", Name: "Zone 1", Code: "A4"}); err != nil {
		t.Fatal(err)
	}
	return httpapi.New(seasons, sites, devices, contacts,
		&relaypub.Client{RelayBase: "http://127.0.0.1:18080"},
		nil,
		httpapi.Options{Version: "test", APIToken: token},
	)
}

func TestPutSite_MalformedJSON(t *testing.T) {
	s := newSitesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/v1/sites/site1", strings.NewReader(`{"name":`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPutSite_WrongToken(t *testing.T) {
	s := newSitesTestServer(t, "secret")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/sites/site1", strings.NewReader(`{"name":"Renamed"}`))
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteSite_HappyPathAndNotFound(t *testing.T) {
	s := newSitesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/sites/site1", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("delete want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/sites/site1", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("second delete want 404, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteSite_WrongToken(t *testing.T) {
	s := newSitesTestServer(t, "secret")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/sites/site1", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPutTelemetry_MalformedJSON(t *testing.T) {
	s := newSitesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/v1/zones/z1/telemetry", strings.NewReader(`{"url":`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPutTelemetry_MissingURL(t *testing.T) {
	s := newSitesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/v1/zones/z1/telemetry", strings.NewReader(`{}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing url, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPutTelemetry_WrongToken(t *testing.T) {
	s := newSitesTestServer(t, "secret")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/zones/z1/telemetry", strings.NewReader(`{"url":"http://example/probe"}`))
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteTelemetry_HappyPath(t *testing.T) {
	s := newSitesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/v1/zones/z1/telemetry", strings.NewReader(`{"url":"http://example/probe"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("put telemetry want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/zones/z1/telemetry", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("delete telemetry want 200, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteTelemetry_WrongToken(t *testing.T) {
	s := newSitesTestServer(t, "secret")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/zones/z1/telemetry", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rr.Code, rr.Body.String())
	}
}
