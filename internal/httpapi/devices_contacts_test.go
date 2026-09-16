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

func newDevicesContactsServer(t *testing.T, token string) (*httpapi.Server, string) {
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
	s := httpapi.New(seasons, sites, devices, contacts,
		&relaypub.Client{RelayBase: "http://127.0.0.1:18080"},
		nil,
		httpapi.Options{Version: "test", APIToken: token},
	)
	return s, "z1"
}

func do(s *httpapi.Server, method, path, token, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func TestDeviceCRUD_HappyPath(t *testing.T) {
	s, zoneID := newDevicesContactsServer(t, "")

	rr := do(s, http.MethodPost, "/v1/devices", "", `{"id":"dev1","name":"Pump","zone_id":"`+zoneID+`"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(s, http.MethodGet, "/v1/devices/dev1", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"name":"Pump"`) {
		t.Fatalf("get want 200 w/ Pump, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(s, http.MethodPut, "/v1/devices/dev1", "", `{"name":"Pump2","zone_id":"`+zoneID+`"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"name":"Pump2"`) {
		t.Fatalf("put want 200 w/ Pump2, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(s, http.MethodDelete, "/v1/devices/dev1", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("delete want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(s, http.MethodGet, "/v1/devices/dev1", "", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get after delete want 404, got %d", rr.Code)
	}
}

func TestContactCRUD_HappyPath(t *testing.T) {
	s, _ := newDevicesContactsServer(t, "")

	rr := do(s, http.MethodPost, "/v1/contacts", "", `{"id":"c1","name":"Op","role":"operator"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(s, http.MethodGet, "/v1/contacts/c1", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"role":"operator"`) {
		t.Fatalf("get want 200 w/ operator, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(s, http.MethodPut, "/v1/contacts/c1", "", `{"name":"Op2","role":"agronomist"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"role":"agronomist"`) {
		t.Fatalf("put want 200 w/ agronomist, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(s, http.MethodDelete, "/v1/contacts/c1", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("delete want 200, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(s, http.MethodGet, "/v1/contacts/c1", "", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get after delete want 404, got %d", rr.Code)
	}
}

func TestCreateDevice_MalformedJSON(t *testing.T) {
	s, _ := newDevicesContactsServer(t, "")
	rr := do(s, http.MethodPost, "/v1/devices", "", `{"id":`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestCreateContact_MalformedJSON(t *testing.T) {
	s, _ := newDevicesContactsServer(t, "")
	rr := do(s, http.MethodPost, "/v1/contacts", "", `{"id":`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestCreateDevice_UnknownZone(t *testing.T) {
	s, _ := newDevicesContactsServer(t, "")
	rr := do(s, http.MethodPost, "/v1/devices", "", `{"id":"dev1","name":"Pump","zone_id":"does-not-exist"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "zone_id not found") {
		t.Fatalf("want 400 zone_id not found, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestDevicesContacts_WrongToken(t *testing.T) {
	s, zoneID := newDevicesContactsServer(t, "secret")
	rr := do(s, http.MethodPut, "/v1/devices/dev1", "wrong-token", `{"name":"Pump","zone_id":"`+zoneID+`"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteDevice_NotFound(t *testing.T) {
	s, _ := newDevicesContactsServer(t, "")
	rr := do(s, http.MethodDelete, "/v1/devices/missing", "", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteContact_NotFound(t *testing.T) {
	s, _ := newDevicesContactsServer(t, "")
	rr := do(s, http.MethodDelete, "/v1/contacts/missing", "", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rr.Code, rr.Body.String())
	}
}
