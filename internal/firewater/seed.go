// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater

import (
	"time"

	"github.com/zyvorai/relay-edge/internal/contact"
	"github.com/zyvorai/relay-edge/internal/device"
	"github.com/zyvorai/relay-edge/internal/season"
	"github.com/zyvorai/relay-edge/internal/site"
)

type SeedResult struct {
	Site    site.Site       `json:"site"`
	Zone    site.Zone       `json:"zone"`
	Season  season.Season   `json:"season"`
	Contact contact.Contact `json:"contact"`
	Devices []device.Device `json:"devices"`
}

func Seed(sites *site.Store, devices *device.Store, contacts *contact.Store, seasons *season.Store) (SeedResult, error) {
	var out SeedResult
	sit, err := sites.PutSite(site.Site{
		ID:     SiteID,
		Name:   "Nashik industrial plant",
		Labels: map[string]string{"domain": "industrial_firewater", "region": "maharashtra"},
	})
	if err != nil {
		return out, err
	}
	out.Site = sit

	z, err := sites.PutZone(site.Zone{
		ID:     ZoneID,
		SiteID: SiteID,
		Name:   "Process block A — fire water",
		Code:   ZoneCode,
		Labels: map[string]string{"domain": "industrial_firewater"},
	})
	if err != nil {
		return out, err
	}
	out.Zone = z

	if _, err := sites.SetTelemetry(ZoneID, &site.VerificationProbe{
		URL:      "http://127.0.0.1:18086/v1/firewater/snapshot",
		Method:   "GET",
		JSONPath: "$.pump_on",
		Expect:   "true",
	}); err != nil {
		return out, err
	}

	c, err := contacts.Put(contact.Contact{
		ID:    ContactID,
		Name:  "EHS duty officer",
		Role:  "ehs",
		SMS:   "+910000000911",
		Email: "ehs@plant.local",
	})
	if err != nil {
		return out, err
	}
	out.Contact = c
	if _, err := sites.SetRouting(SiteID, map[string]string{
		"ehs":          ContactID,
		"operator":     ContactID,
		"control_room": ContactID,
	}); err != nil {
		return out, err
	}

	now := time.Now().UTC()
	sea, err := seasons.Put(season.Season{
		ID:       SeasonID,
		Name:     "FY26 fire-water watch",
		Crop:     "fire-water",
		SiteID:   SiteID,
		Site:     sit.Name,
		Stage:    "idle",
		Status:   "planned",
		Labels:   map[string]string{"domain": "industrial_firewater"},
		Notes:    "Industrial fire-water watch window used by the /ui simulator.",
		StartsAt: now,
		EndsAt:   now.Add(365 * 24 * time.Hour),
	})
	if err != nil {
		return out, err
	}
	sea, err = seasons.UpdateStatus(SeasonID, "active", "")
	if err != nil {
		return out, err
	}
	out.Season = sea

	devs := []device.Device{{
		ID: GatewayID, ZoneID: ZoneID, Name: "Fire-water edge gateway",
		Kind: "hub", ExternalID: "fw-gw-01", Commands: []string{},
		Labels: map[string]string{"metric": "gateway"},
	}}
	seen := map[string]bool{GatewayID: true}
	for _, sdef := range Catalog() {
		if seen[sdef.Device] {
			continue
		}
		seen[sdef.Device] = true
		devs = append(devs, device.Device{
			ID: sdef.Device, ZoneID: ZoneID, Name: sdef.Name,
			Kind: sdef.Kind, ExternalID: sdef.Device, Commands: CommandsFor(sdef.Kind, sdef.Class),
			Labels: map[string]string{
				"metric": sdef.ID, "unit": sdef.Unit, "domain": "industrial_firewater",
				"class": sdef.Class, "protocol": sdef.Protocol, "vendor": sdef.Vendor,
			},
		})
	}
	for _, d := range devs {
		got, err := devices.Put(d)
		if err != nil {
			return out, err
		}
		out.Devices = append(out.Devices, got)
	}
	return out, nil
}
