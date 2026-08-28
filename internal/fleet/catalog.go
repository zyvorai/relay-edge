// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0
//
// Master edge-class catalog. One row per *class of box* a 2026 industrial
// site actually hangs off an edge node — not every OEM SKU.

package fleet

type Dev struct {
	ID, Name, Class, Domain, Protocol, Unit string
	Min, Max, Nominal                       float64
	Flag                                    bool
}

func Catalog() []Dev {
	d := func(id, name, class, domain, proto, unit string, min, max, nom float64) Dev {
		return Dev{ID: id, Name: name, Class: class, Domain: domain, Protocol: proto, Unit: unit, Min: min, Max: max, Nominal: nom, Flag: unit == "flag"}
	}
	return []Dev{
		// --- robots / mobility ---
		d("amr_batt", "AMR battery", "robot", "logistics", "VDA-5050", "%", 0, 100, 72),
		d("amr_pose", "AMR localized", "robot", "logistics", "VDA-5050", "flag", 0, 1, 1),
		d("agv_load", "AGV payload", "robot", "logistics", "CAN", "kg", 0, 1500, 0),
		d("fork_load", "Smart forklift load", "robot", "logistics", "CAN", "kg", 0, 2500, 0),
		d("cobot_nm", "Cobot joint torque", "robot", "discrete", "EtherCAT", "N·m", 0, 80, 12),
		d("crane_t", "Yard crane load", "robot", "yard", "Modbus", "t", 0, 40, 0),

		// --- RTLS / identity ---
		d("uwb_anchor", "UWB anchor health", "rtls", "people", "UWB", "flag", 0, 1, 1),
		d("ble_tags", "BLE tags heard", "rtls", "people", "BLE", "n", 0, 400, 86),
		d("rfid_gate", "RFID dock gate", "rtls", "logistics", "LLRP", "reads/s", 0, 40, 0),
		d("anpr_ok", "LPR / ANPR camera", "identity", "yard", "ONVIF", "flag", 0, 1, 1),
		d("face_gate", "Biometric turnstile", "identity", "access", "OSDP", "flag", 0, 1, 1),
		d("epaper", "Andon / e-paper", "identity", "discrete", "BLE", "flag", 0, 1, 1),

		// --- wearables / first responders ---
		d("bodycam", "Bodycam recording", "wearable", "security", "Wi-Fi6", "flag", 0, 1, 0),
		d("helmet_gas", "Smart-helmet LEL", "wearable", "safety", "BLE", "%LEL", 0, 100, 0.3),
		d("radio_rssi", "TETRA / P25 radio", "wearable", "safety", "P25", "dBm", -120, -50, -78),
		d("watch_hr", "Worker HR (opt-in)", "wearable", "safety", "BLE", "bpm", 40, 200, 78),

		// --- energy / microgrid ---
		d("pv_kw", "Rooftop PV", "energy", "power", "SunSpec", "kW", 0, 250, 40),
		d("bess_soc", "BESS state of charge", "energy", "power", "Modbus", "%", 0, 100, 64),
		d("bess_c", "BESS cell temp", "energy", "power", "CAN", "°C", 0, 60, 29),
		d("evse_kw", "Fleet EV charger", "energy", "power", "OCPP", "kW", 0, 150, 0),
		d("meter_kw", "Smart meter import", "energy", "power", "DLMS", "kW", 0, 800, 120),
		d("inverter_ok", "PV inverter", "energy", "power", "SunSpec", "flag", 0, 1, 1),
		d("xfmr_c", "Yard transformer oil", "energy", "power", "DNP3", "°C", 0, 110, 58),
		d("rmu_ok", "Ring-main unit", "energy", "power", "IEC61850", "flag", 0, 1, 1),

		// --- building / BMS ---
		d("chiller_kw", "Chiller electric", "building", "bms", "BACnet", "kW", 0, 400, 90),
		d("ahu_pa", "AHU filter ΔP", "building", "bms", "BACnet", "Pa", 0, 400, 80),
		d("vav_pct", "VAV box position", "building", "bms", "BACnet", "%", 0, 100, 45),
		d("lift_ok", "Elevator controller", "building", "bms", "BACnet", "flag", 0, 1, 1),
		d("lux", "Smart lighting lux", "building", "bms", "DALI", "lx", 0, 1000, 320),
		d("crac_c", "Galleon CRAC supply", "building", "datacenter", "SNMP", "°C", 10, 35, 22),
		d("cdu_lpm", "Liquid CDU flow", "building", "datacenter", "Modbus", "L/min", 0, 80, 18),

		// --- OT gateways / fieldbus ---
		d("mb_gw", "Modbus TCP gateway", "ot_gw", "ot", "Modbus", "flag", 0, 1, 1),
		d("opc_gw", "OPC-UA gateway", "ot_gw", "ot", "OPC-UA", "flag", 0, 1, 1),
		d("pn_gw", "PROFINET coupler", "ot_gw", "ot", "PROFINET", "flag", 0, 1, 1),
		d("hart_gw", "WirelessHART GW", "ot_gw", "ot", "HART", "flag", 0, 1, 1),
		d("dnp_gw", "DNP3 outstation", "ot_gw", "ot", "DNP3", "flag", 0, 1, 1),
		d("iec_gw", "IEC 61850 gateway", "ot_gw", "ot", "IEC61850", "flag", 0, 1, 1),
		d("spark_gw", "Sparkplug host", "ot_gw", "ot", "MQTT", "flag", 0, 1, 1),

		// --- discrete machines ---
		d("cnc_ok", "CNC cell", "machine", "discrete", "MTConnect", "flag", 0, 1, 1),
		d("cnc_oee", "CNC OEE", "machine", "discrete", "MTConnect", "%", 0, 100, 68),
		d("weld_a", "Weld cell current", "machine", "discrete", "EtherNet/IP", "A", 0, 400, 0),
		d("conv_mps", "Conveyor speed", "machine", "discrete", "PROFINET", "m/s", 0, 3, 0.8),
		d("press_bar", "Hydraulic press", "machine", "discrete", "IO-Link", "bar", 0, 250, 0),

		// --- water / wastewater (beyond fire-water) ---
		d("ww_do", "Aeration DO", "water", "wwtp", "Modbus", "mg/L", 0, 12, 2.4),
		d("ww_cl2", "Chlorine residual", "water", "wwtp", "4-20mA", "mg/L", 0, 5, 0.6),
		d("ww_turb", "Effluent NTU", "water", "wwtp", "SDI-12", "NTU", 0, 40, 1.2),
		d("ww_inf", "Influent flow", "water", "wwtp", "Modbus", "m3/h", 0, 800, 210),
		d("cp_mv", "Pipe CP potential", "water", "pipeline", "4-20mA", "mV", -2000, 0, -850),

		// --- environment / CEMS ---
		d("aqi_pm", "Yard PM2.5", "env", "ehs", "LoRaWAN", "µg/m3", 0, 300, 28),
		d("cems_nox", "Stack NOx", "env", "ehs", "Modbus", "ppm", 0, 400, 42),
		d("noise_db", "Boundary noise", "env", "ehs", "LoRaWAN", "dBA", 30, 110, 61),
		d("seismic", "Blast / seismic", "env", "mine", "RS485", "mm/s", 0, 50, 0.2),
		d("lightn", "Lightning alarm", "env", "yard", "GPIO", "flag", 0, 1, 0),

		// --- rail / yard ---
		d("hotbox", "Hot-box detector", "rail", "yard", "Ethernet", "°C", 0, 200, 48),
		d("axle", "Axle counter", "rail", "yard", "Vital", "flag", 0, 1, 1),
		d("reefer_c", "Container reefer", "rail", "yard", "LTE-M", "°C", -30, 30, -18),
		d("wb_t", "Weighbridge", "rail", "yard", "RS232", "t", 0, 80, 0),

		// --- agri (original relay-edge domain) ---
		d("soil_vwc", "Soil VWC", "agri", "farm", "LoRaWAN", "%", 0, 60, 28),
		d("soil_ec", "Soil EC", "agri", "farm", "LoRaWAN", "dS/m", 0, 8, 1.1),
		d("leaf_c", "Canopy temp", "agri", "farm", "LoRaWAN", "°C", -5, 50, 26),
		d("irr_open", "Pivot / valve", "agri", "farm", "LoRaWAN", "%", 0, 100, 0),
		d("herd_tag", "Livestock tags", "agri", "farm", "BLE", "n", 0, 500, 120),

		// --- marine / remote ---
		d("ais_ok", "AIS receiver", "marine", "port", "NMEA", "flag", 0, 1, 1),
		d("tide_m", "Tide gauge", "marine", "port", "SDI-12", "m", 0, 8, 2.1),
		d("iridium", "Iridium SBD", "satcom", "remote", "SBD", "flag", 0, 1, 1),

		// --- public-safety radio / DAS ---
		d("das_ok", "Public-safety DAS", "ps_radio", "life", "SNMP", "flag", 0, 1, 1),
		d("elight", "Emergency lighting", "life", "life", "DALI", "flag", 0, 1, 1),
		d("aed_ok", "Connected AED cabinet", "life", "life", "NB-IoT", "flag", 0, 1, 1),
		d("ext_cap", "Extinguisher cap", "life", "life", "LoRaWAN", "flag", 0, 1, 1),
		d("refuge", "Mine refuge chamber", "life", "mine", "RS485", "flag", 0, 1, 1),

		// --- comms extras ---
		d("wifi_ap", "Wi-Fi 6E AP", "radio", "it", "CAPWAP", "flag", 0, 1, 1),
		d("wifi_cli", "Wi-Fi clients", "radio", "it", "CAPWAP", "n", 0, 200, 34),
		d("otdr", "Fiber OTDR event", "radio", "it", "SNMP", "flag", 0, 1, 0),
		d("gnss_ok", "Site GNSS clock", "radio", "it", "NMEA", "flag", 0, 1, 1),

		// --- security extras ---
		d("hsm_ok", "Edge HSM / TPM", "security", "it", "PKCS11", "flag", 0, 1, 1),
		d("cert_days", "Device cert days left", "security", "it", "EST", "d", 0, 730, 240),
		d("ids_evt", "OT IDS events", "security", "it", "Syslog", "n/min", 0, 50, 0),
	}
}

func Classes() []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range Catalog() {
		if seen[x.Class] {
			continue
		}
		seen[x.Class] = true
		out = append(out, x.Class)
	}
	return out
}
