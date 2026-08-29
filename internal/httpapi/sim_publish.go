// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"fmt"
	"log"
	"time"

	"github.com/zyvorai/relay-edge/internal/firewater"
)

// publishSimEvent stamps and publishes a simulator event into Relay (gateway or direct).
func (s *Server) publishSimEvent(evType, severity, command, deviceID, domain string, data map[string]any) {
	it, err := s.Seasons.Get(firewater.SeasonID)
	if err != nil {
		log.Printf("%s publish: season %s missing (POST /v1/firewater/seed first)", domain, firewater.SeasonID)
		return
	}
	ctx := s.resolveEnrich(it, firewater.ZoneID, firewater.ZoneCode, deviceID)
	key := fmt.Sprintf("edge/%s/%s/%s/%d", domain, evType, deviceID, time.Now().UnixNano())
	extra := data
	if extra == nil {
		extra = map[string]any{}
	}
	extra["sim_domain"] = domain
	if command != "" {
		target := domain + "-controller"
		extra["recommended_action"] = map[string]any{
			"target":  target,
			"command": command,
			"payload": map[string]any{
				"zone":      firewater.ZoneCode,
				"zone_id":   firewater.ZoneID,
				"device_id": deviceID,
				"season_id": it.ID,
			},
		}
	}
	stamped := s.stampData(ctx, extra)
	s.IncPublish()
	if _, err := s.Pub.PublishEventType(evType, severity, seasonSource(ctx), key, stamped); err != nil {
		log.Printf("%s publish %s: %v", domain, evType, err)
	}
}
