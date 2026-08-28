// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater

// Node is one asset on the fire-water network (not a raw sample).
type Node struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // tank|pump|header|valve|riser|hydrant|source
	Building string `json:"building"`
	Metric   string `json:"metric,omitempty"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Via  string `json:"via,omitempty"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

func PlantGraph() Graph {
	return Graph{
		Nodes: []Node{
			{ID: "src_municipal", Name: "Municipal inlet", Kind: "source", Building: "yard"},
			{ID: "tank_main", Name: "Fire tank", Kind: "tank", Building: "pump_house", Metric: "tank_level"},
			{ID: "jockey", Name: "Jockey pump", Kind: "pump", Building: "pump_house", Metric: "jockey_bar"},
			{ID: "main_pump", Name: "Main electric pump", Kind: "pump", Building: "pump_house", Metric: "main_bar"},
			{ID: "diesel_pump", Name: "Diesel fire pump", Kind: "pump", Building: "pump_house", Metric: "diesel_ecu"},
			{ID: "header", Name: "Discharge header", Kind: "header", Building: "pump_house", Metric: "header_lps"},
			{ID: "valve_a", Name: "Process A OS&Y", Kind: "valve", Building: "process_a", Metric: "valve_open"},
			{ID: "riser_a", Name: "Process A remote riser", Kind: "riser", Building: "process_a", Metric: "riser_bar"},
			{ID: "hydrant_yard", Name: "Yard hydrant H-12", Kind: "hydrant", Building: "yard", Metric: "hydrant_bar"},
			{ID: "admin_riser", Name: "Admin block riser", Kind: "riser", Building: "admin", Metric: "riser_bar"},
		},
		Edges: []Edge{
			{From: "src_municipal", To: "tank_main"},
			{From: "tank_main", To: "jockey"},
			{From: "tank_main", To: "main_pump"},
			{From: "tank_main", To: "diesel_pump"},
			{From: "jockey", To: "header"},
			{From: "main_pump", To: "header"},
			{From: "diesel_pump", To: "header"},
			{From: "header", To: "valve_a"},
			{From: "valve_a", To: "riser_a"},
			{From: "header", To: "hydrant_yard"},
			{From: "header", To: "admin_riser"},
		},
	}
}

// Downstream reports assets that go dry if `id` is isolated.
func (g Graph) Downstream(id string) []string {
	seen := map[string]bool{id: true}
	var out []string
	changed := true
	for changed {
		changed = false
		for _, e := range g.Edges {
			if seen[e.From] && !seen[e.To] {
				seen[e.To] = true
				out = append(out, e.To)
				changed = true
			}
		}
	}
	return out
}
