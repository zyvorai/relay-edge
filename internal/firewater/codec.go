// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater

import "time"

// Sparkplug-style NBIRTH/NDATA payload (subset) so a gateway can emit
// the same points Eclipse Tahu / Ignition expect.
type SparkplugMetric struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type SparkplugPayload struct {
	Timestamp int64             `json:"timestamp"`
	Seq       int               `json:"seq"`
	UUID      string            `json:"uuid"`
	Metrics   []SparkplugMetric `json:"metrics"`
}

func Sparkplug(group, node string, seq int, v map[string]float64) map[string]any {
	pl := SparkplugPayload{
		Timestamp: time.Now().UnixMilli(),
		Seq:       seq,
		UUID:      node,
	}
	for _, p := range Catalog() {
		typ := "Double"
		var val any = round(v[p.ID])
		if p.Unit == "flag" {
			typ = "Boolean"
			val = v[p.ID] >= 0.5
		}
		pl.Metrics = append(pl.Metrics, SparkplugMetric{Name: p.ID, Type: typ, Value: val})
	}
	return map[string]any{
		"topic":   "spBv1.0/" + group + "/NDATA/" + node,
		"payload": pl,
	}
}

type ModbusReg struct {
	Name    string  `json:"name"`
	Address int     `json:"address"`
	Unit    string  `json:"unit"`
	Scale   float64 `json:"scale"`
	Raw     uint16  `json:"raw"`
	Value   float64 `json:"value"`
}

func ModbusMap(v map[string]float64) []ModbusReg {
	out := make([]ModbusReg, 0, len(Catalog()))
	addr := 40001
	for _, p := range Catalog() {
		scale := 10.0
		if p.Unit == "flag" {
			scale = 1
		}
		raw := uint16(v[p.ID] * scale)
		out = append(out, ModbusReg{
			Name: p.ID, Address: addr, Unit: p.Unit, Scale: scale, Raw: raw, Value: round(v[p.ID]),
		})
		addr++
	}
	return out
}
