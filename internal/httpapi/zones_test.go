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

func newZonesTestServer(t *testing.T, token string) *httpapi.Server {
	t.Helper()
	dir := t.TempDir()
	seasons, _ := season.Open(filepath.Join(dir, "seasons.json"))
	sites, _ := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	devices, _ := device.Open(filepath.Join(dir, "devices.json"))
	contacts, _ := contact.Open(filepath.Join(dir, "contacts.json"))
	if _, err := sites.PutSite(site.Site{ID: "site1", Name: "Site 1"}); err != nil {
		t.Fatal(err)
	}
	return httpapi.New(seasons, sites, devices, contacts,
		&relaypub.Client{RelayBase: "http://127.0.0.1:18080"},
		nil,
		httpapi.Options{Version: "test", APIToken: token},
	)
}

func TestZoneCRUD_HappyPath(t *testing.T) {
	s := newZonesTestServer(t, "")

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/sites/site1/zones", strings.NewReader(`{"id":"z1","name":"Zone 1","code":"A4"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/zones/z1", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"code":"A4"`) {
		t.Fatalf("get want 200 w/ A4, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sites/site1/zones", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"z1"`) {
		t.Fatalf("list by site want 200 w/ z1, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/v1/zones/z1", strings.NewReader(`{"name":"Zone 1 Renamed","code":"A5"}`)))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"code":"A5"`) {
		t.Fatalf("put want 200 w/ A5, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/zones/z1", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("delete want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/zones/z1", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get after delete want 404, got %d", rr.Code)
	}
}

func TestCreateZone_MalformedJSON(t *testing.T) {
	s := newZonesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/sites/site1/zones", strings.NewReader(`{"name":`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestCreateZone_UnknownSite(t *testing.T) {
	s := newZonesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/sites/does-not-exist/zones", strings.NewReader(`{"id":"z1","name":"Zone 1"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestZone_WrongToken(t *testing.T) {
	s := newZonesTestServer(t, "secret")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sites/site1/zones", strings.NewReader(`{"id":"z1","name":"Zone 1"}`))
	req.Header.Set("Authorization", "Bearer wrong")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteZone_NotFound(t *testing.T) {
	s := newZonesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/zones/missing", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestListAllZones(t *testing.T) {
	s := newZonesTestServer(t, "")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/sites/site1/zones", strings.NewReader(`{"id":"z1","name":"Zone 1"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/zones?site_id=site1", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"z1"`) {
		t.Fatalf("want 200 w/ z1, got %d %s", rr.Code, rr.Body.String())
	}
}
