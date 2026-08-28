// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0
//
// Atlas-class asset catalog. Names describe the *kind of gear* a remote-edge
// NOC (Armada Atlas / similar) actually tracks — we do not implement Armada.

package atlas

type Asset struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Class    string  `json:"class"`
	Vendor   string  `json:"vendor"`
	Site     string  `json:"site"`
	Metric   string  `json:"metric"`
	Unit     string  `json:"unit"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Protocol string  `json:"protocol"`
}

func Catalog() []Asset {
	return []Asset{
		// Galleon-class compute (suitcase → container → MW)
		{"gal_beacon", "Beacon suitcase node", "galleon", "edge-box", "rail_yard", "beacon_ok", "flag", 0, 1, "MQTT"},
		{"gal_cpu", "Beacon CPU", "galleon", "edge-box", "rail_yard", "beacon_cpu", "%", 0, 100, "MQTT"},
		{"gal_gpu", "Cruiser GPU util", "galleon", "gpu", "rail_yard", "cruiser_gpu", "%", 0, 100, "NVML"},
		{"gal_vram", "Cruiser VRAM", "galleon", "gpu", "rail_yard", "cruiser_vram", "%", 0, 100, "NVML"},
		{"gal_temp", "Cruiser intake", "galleon", "thermal", "rail_yard", "cruiser_c", "°C", 0, 60, "IPMI"},
		{"gal_pwr", "Cruiser draw", "galleon", "power", "rail_yard", "cruiser_kw", "kW", 0, 80, "SNMP"},
		{"gal_k8s", "AEP / K3s ready", "galleon", "runtime", "rail_yard", "k3s_ok", "flag", 0, 1, "gRPC"},

		// Connect — Starlink + SD-WAN + LTE/5G
		{"sat_snr", "Starlink SNR", "starlink", "sat", "rail_yard", "starlink_snr", "dB", 0, 12, "gRPC"},
		{"sat_obstr", "Starlink obstruction", "starlink", "sat", "rail_yard", "starlink_obstr", "%", 0, 100, "gRPC"},
		{"sat_lat", "Starlink RTT", "starlink", "sat", "rail_yard", "starlink_ms", "ms", 0, 800, "gRPC"},
		{"wan_ok", "SD-WAN overlay", "sdwan", "wan", "rail_yard", "sdwan_ok", "flag", 0, 1, "NETCONF"},
		{"lte_rsrp", "LTE backup RSRP", "cellular", "lte", "rail_yard", "lte_rsrp", "dBm", -140, -50, "AT"},
		{"p5g_ue", "Private 5G UEs", "p5g", "ran", "rail_yard", "p5g_ue", "ue", 0, 64, "O1"},
		{"p5g_prb", "Private 5G PRB", "p5g", "ran", "rail_yard", "p5g_prb", "%", 0, 100, "O1"},

		// Drones + vision (OpsAI-class workloads)
		{"uav_batt", "Yard drone battery", "drone", "uav", "rail_yard", "drone_batt", "%", 0, 100, "MAVLink"},
		{"uav_alt", "Yard drone AGL", "drone", "uav", "rail_yard", "drone_alt", "m", 0, 120, "MAVLink"},
		{"uav_link", "Drone C2 link", "drone", "uav", "rail_yard", "drone_link", "flag", 0, 1, "MAVLink"},
		{"cam_fps", "Perimeter cam infer", "vision", "camera", "rail_yard", "cam_fps", "fps", 0, 30, "RTSP"},
		{"ppe_score", "PPE / safety score", "vision", "camera", "rail_yard", "ppe_score", "%", 0, 100, "NPU"},
		{"intrude", "Perimeter intrusion", "vision", "camera", "rail_yard", "intrude", "flag", 0, 1, "NPU"},

		// Industrial IoT already typical at the same site
		{"wx_c", "Yard weather", "iot", "weather", "rail_yard", "wx_c", "°C", -20, 50, "LoRaWAN"},
		{"flood", "Yard flood", "iot", "flood", "rail_yard", "flood_mm", "mm", 0, 500, "LoRaWAN"},
		{"gps_fix", "Asset tracker fix", "iot", "gps", "rail_yard", "gps_fix", "flag", 0, 1, "LTE-M"},
		{"fw_ready", "Fire-water ready", "iot", "safety", "rail_yard", "fw_ready", "flag", 0, 1, "MQTT"},
	}
}
