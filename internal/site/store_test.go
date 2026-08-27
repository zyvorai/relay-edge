// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package site_test

import (
	"path/filepath"
	"testing"

	"github.com/zyvorai/relay-edge/internal/site"
)

func TestSitesZonesTelemetry(t *testing.T) {
	dir := t.TempDir()
	st, err := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	if err != nil {
		t.Fatal(err)
	}
	sit, err := st.PutSite(site.Site{ID: "farm-184", Name: "Farm 184"})
	if err != nil || !sit.Active {
		t.Fatalf("site: %+v %v", sit, err)
	}
	z, err := st.PutZone(site.Zone{ID: "z-a4", SiteID: "farm-184", Name: "Block A4", Code: "A4"})
	if err != nil || z.Code != "A4" {
		t.Fatalf("zone: %+v %v", z, err)
	}
	z2, err := st.SetTelemetry("z-a4", &site.VerificationProbe{
		URL: "http://127.0.0.1:8091/v1/telemetry/A4", JSONPath: "$.ok", Expect: "true",
	})
	if err != nil || z2.Telemetry == nil || z2.Telemetry.URL == "" {
		t.Fatalf("telemetry: %+v %v", z2, err)
	}
	_, err = st.SetRouting("farm-184", map[string]string{"farmer": "c1"})
	if err != nil {
		t.Fatal(err)
	}
	st2, err := site.Open(filepath.Join(dir, "sites.json"), filepath.Join(dir, "zones.json"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := st2.GetZone("z-a4")
	if err != nil || again.Telemetry == nil {
		t.Fatalf("reload: %+v %v", again, err)
	}
}
