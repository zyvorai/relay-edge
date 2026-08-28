// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package firewater

// Classes of gear that actually ships on industrial / fire-water edge nodes today.
const (
	ClassSense   = "sense"
	ClassActuate = "actuate"
	ClassControl = "control"
	ClassCompute = "compute"
	ClassRadio   = "radio"
	ClassVision  = "vision"
	ClassPower   = "power"
	ClassSafety  = "safety"
	ClassAccess  = "access"
)

func Catalog() []SensorDef {
	w := func(v float64) *float64 { return &v }
	return []SensorDef{
		// --- sense ---
		p("tank_level", "Fire tank / pond level", "%", 0, 100, w(50), w(30), false, "dev_fw_tank_lt01", "sensor", ClassSense, "4-20mA", "Wika"),
		p("jockey_bar", "Jockey discharge pressure", "bar", 0, 16, w(6.5), w(5.5), true, "dev_fw_jockey_pt01", "sensor", ClassSense, "Modbus-RTU", "Danfoss"),
		p("main_bar", "Main pump discharge pressure", "bar", 0, 16, w(6.0), w(4.5), true, "dev_fw_pump_pt01", "sensor", ClassSense, "Modbus-RTU", "Danfoss"),
		p("riser_bar", "Most-remote riser pressure", "bar", 0, 16, w(5.5), w(4.0), true, "dev_fw_riser_pt01", "sensor", ClassSense, "LoRaWAN", "Milesight"),
		p("header_lps", "Header electromagnetic flow", "L/s", 0, 80, nil, nil, false, "dev_fw_flow_ft01", "sensor", ClassSense, "Modbus-TCP", "Krohne"),
		p("hydrant_bar", "Yard hydrant pressure", "bar", 0, 16, w(4.5), w(3.0), true, "dev_fw_hydrant_pt01", "hydrant", ClassSense, "NB-IoT", "McWane"),
		p("room_c", "Pump room ambient", "°C", -5, 50, w(8), w(4), true, "dev_fw_room_tt01", "sensor", ClassSense, "BLE", "Sensirion"),
		p("flood_mm", "Pump room flood / sump", "mm", 0, 200, w(10), w(30), false, "dev_fw_room_lt02", "sensor", ClassSense, "RS485", "VEGA"),
		p("diesel_pct", "Diesel day-tank level", "%", 0, 100, w(40), w(20), false, "dev_fw_diesel_lt01", "sensor", ClassSense, "4-20mA", "Emerson"),
		p("pump_amps", "Main pump current", "A", 0, 220, nil, nil, false, "dev_fw_pump_ct01", "sensor", ClassSense, "Modbus-TCP", "Schneider"),
		p("pump_vib", "Pump housing vibration", "mm/s", 0, 20, w(7), w(11), false, "dev_fw_pump_vt01", "sensor", ClassSense, "IO-Link", "SKF"),
		p("leak_db", "Acoustic leak level", "dB", 20, 100, w(62), w(75), false, "dev_fw_leak_ae01", "sensor", ClassSense, "LoRaWAN", "Gutermann"),
		p("turbidity", "Stored-water turbidity", "NTU", 0, 40, w(8), w(15), false, "dev_fw_wq_tu01", "sensor", ClassSense, "SDi-12", "Hach"),
		p("tamper", "Hydrant cap tamper", "flag", 0, 1, w(1), w(1), false, "dev_fw_hydrant_ts01", "hydrant", ClassSense, "NB-IoT", "McWane"),

		// --- actuate ---
		p("valve_open", "Zone A OS&Y position", "% open", 0, 100, w(90), w(20), false, "dev_fw_valve_zs01", "valve", ClassActuate, "BACnet", "Tyco"),
		p("deluge_open", "Deluge solenoid", "% open", 0, 100, nil, nil, false, "dev_fw_deluge_sv01", "valve", ClassActuate, "24VDC", "Viking"),
		p("foam_pct", "Foam concentrate", "%", 0, 100, w(25), w(10), false, "dev_fw_foam_skid", "pump", ClassActuate, "Modbus-RTU", "FireDos"),
		p("heater_on", "Pump-room heater", "flag", 0, 1, nil, nil, false, "dev_fw_heater_01", "actuator", ClassActuate, "GPIO", "Chromalox"),
		p("siren_on", "Yard siren / strobe", "flag", 0, 1, nil, nil, false, "dev_fw_siren_01", "actuator", ClassActuate, "GPIO", "Eaton"),
		p("vfd_hz", "Jockey VFD output", "Hz", 0, 60, nil, nil, false, "dev_fw_jockey_vfd", "pump", ClassActuate, "Modbus-TCP", "ABB"),

		// --- control ---
		p("plc_ok", "Pump-house PLC heartbeat", "flag", 0, 1, w(1), w(0), true, "dev_fw_plc_s7", "controller", ClassControl, "OPC-UA", "Siemens"),
		p("facp_ok", "Fire alarm panel", "flag", 0, 1, w(1), w(0), true, "dev_fw_facp_01", "controller", ClassControl, "BACnet", "Notifier"),
		p("diesel_ecu", "Diesel engine ECU ready", "flag", 0, 1, w(1), w(0), true, "dev_fw_diesel_ecu", "controller", ClassControl, "J1939", "Cummins"),
		p("io_pack", "Remote I/O pack health", "flag", 0, 1, w(1), w(0), true, "dev_fw_io_750", "controller", ClassControl, "EtherNet/IP", "Allen-Bradley"),

		// --- compute / edge box ---
		p("edge_cpu", "Edge box CPU", "%", 0, 100, w(80), w(92), false, "dev_fw_edge_ipc", "hub", ClassCompute, "MQTT", "Advantech"),
		p("edge_temp", "Edge box SoC temp", "°C", 20, 95, w(75), w(85), false, "dev_fw_edge_ipc", "hub", ClassCompute, "MQTT", "Advantech"),
		p("model_fps", "On-device infer FPS", "fps", 0, 60, w(8), w(3), true, "dev_fw_jetson", "hub", ClassCompute, "MQTT", "NVIDIA"),
		p("k3s_ok", "K3s / edge runtime", "flag", 0, 1, w(1), w(0), true, "dev_fw_k3s", "hub", ClassCompute, "gRPC", "Rancher"),

		// --- radio / backhaul ---
		p("mqtt_ok", "MQTT session", "flag", 0, 1, w(1), w(0), true, "dev_fw_mqtt_gw", "hub", ClassRadio, "MQTT", "EMQ"),
		p("opcua_ok", "OPC-UA session", "flag", 0, 1, w(1), w(0), true, "dev_fw_opcua", "hub", ClassRadio, "OPC-UA", "Unified Automation"),
		p("lora_rssi", "LoRaWAN RSSI", "dBm", -130, -40, w(-110), w(-120), true, "dev_fw_lora_gw", "hub", ClassRadio, "LoRaWAN", "Semtech"),
		p("cell_rsrp", "5G / LTE RSRP", "dBm", -140, -50, w(-110), w(-120), true, "dev_fw_5g_cpe", "hub", ClassRadio, "5G", "Quectel"),
		p("tsn_ok", "TSN switch fabric", "flag", 0, 1, w(1), w(0), true, "dev_fw_tsn_sw", "hub", ClassRadio, "TSN", "Cisco"),
		p("ptp_lock", "PTP / GPS time lock", "flag", 0, 1, w(1), w(0), true, "dev_fw_ptp", "hub", ClassRadio, "PTP", "Meinberg"),

		// --- vision ---
		p("cam_ai_fire", "Thermal camera fire score", "%", 0, 100, w(40), w(70), false, "dev_fw_thermal_cam", "camera", ClassVision, "RTSP+NPU", "FLIR"),
		p("cam_smoke", "RGB camera smoke score", "%", 0, 100, w(35), w(60), false, "dev_fw_nvr_ai", "camera", ClassVision, "ONVIF", "Hikvision"),
		p("flame_ir", "IR flame detector", "%", 0, 100, w(20), w(50), false, "dev_fw_flame_ir", "sensor", ClassVision, "4-20mA", "Honeywell"),

		// --- power ---
		p("mains_ok", "Utility mains", "flag", 0, 1, w(1), w(0), true, "dev_fw_mains", "sensor", ClassPower, "Modbus-TCP", "Schneider"),
		p("ups_soc", "UPS state of charge", "%", 0, 100, w(40), w(20), false, "dev_fw_ups", "sensor", ClassPower, "SNMP", "Eaton"),
		p("gen_run", "Standby genset running", "flag", 0, 1, nil, nil, false, "dev_fw_genset", "pump", ClassPower, "J1939", "Caterpillar"),
		p("pdu_w", "PDU draw", "W", 0, 4000, w(2800), w(3500), false, "dev_fw_pdu", "sensor", ClassPower, "SNMP", "Raritan"),

		// --- safety atmosphere ---
		p("lel_pct", "Pump-room LEL", "%LEL", 0, 100, w(10), w(20), false, "dev_fw_gas_lel", "sensor", ClassSafety, "4-20mA", "Dräger"),
		p("co_ppm", "Pump-room CO", "ppm", 0, 200, w(35), w(50), false, "dev_fw_gas_co", "sensor", ClassSafety, "4-20mA", "MSA"),
		p("smoke_pct", "Spot smoke obscuration", "%", 0, 100, w(8), w(15), false, "dev_fw_smoke", "sensor", ClassSafety, "BACnet", "System Sensor"),

		// --- access ---
		p("door_secure", "Pump-room door secure", "flag", 0, 1, w(1), w(0), true, "dev_fw_door", "sensor", ClassAccess, "Wiegand", "HID"),
		p("nfc_lock", "Hydrant NFC lock", "flag", 0, 1, w(1), w(0), true, "dev_fw_nfc_lock", "hydrant", ClassAccess, "NFC", "CA-FIRE"),
		p("occupancy", "Muster occupancy", "pax", 0, 20, nil, nil, false, "dev_fw_mmwave", "sensor", ClassAccess, "BLE", "Texas Instruments"),
	}
}

func p(id, name, unit string, min, max float64, warn, crit *float64, invert bool, dev, kind, class, proto, vendor string) SensorDef {
	return SensorDef{
		ID: id, Name: name, Unit: unit, Min: min, Max: max,
		Warn: warn, Crit: crit, Invert: invert, Device: dev,
		Kind: kind, Class: class, Protocol: proto, Vendor: vendor,
	}
}

func CommandsFor(kind, class string) []string {
	switch {
	case kind == "pump" || class == ClassActuate && kind == "pump":
		return []string{"pump.start", "pump.start_backup", "pump.confirm_run"}
	case kind == "valve":
		return []string{"valve.open", "deluge.release"}
	case kind == "actuator":
		return []string{"siren.on", "heater.on"}
	case kind == "camera":
		return []string{"camera.ptz", "nvr.clip"}
	case kind == "controller":
		return []string{"plc.reset", "facp.ack"}
	default:
		return []string{}
	}
}
