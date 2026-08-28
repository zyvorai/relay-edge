# Simulators

Three companion simulators ship inside relay-edge — each with a web UI, REST API, and optional Relay publish. All share the same stamp pipeline and season context.

← [Docs hub](README.md)

---

## Overview

| Simulator | UI | Catalog size | Publish config |
|-----------|-----|--------------|----------------|
| **Firewater** | [/ui](../web/index.html) | 47 sensor points | `POST /v1/firewater/config` |
| **Remote edge** | [/ui/remote-edge.html](../web/remote-edge.html) | 24 assets | `POST /v1/remote-edge/config` |
| **Fleet** | [/ui/fleet.html](../web/fleet.html) | 77 device classes (15 class chips) | `POST /v1/fleet/config` |

**Shared controls:** interval, **Start stream / Stop / One tick**, scenario picker, SSE event stream, events log.

**Seed (once per data dir):** only firewater exposes `POST /v1/firewater/seed`. It creates site `site_fw_plant`, zone `zone_fw_process_a`, season `season_fw_watch`, and contacts/devices the other simulators reuse. Remote-edge and fleet UIs do not have their own seed — run firewater seed first (UI button or curl) before enabling publish on any simulator.

**Publish default:** `publish: false`. Events appear locally in SSE + `/events` until you set `"publish": true`.

---

## Firewater — industrial fire-water plant

Models a full NFPA-style plant: tanks, pumps, deluge, diesel, freeze protection, plus edge IPC, thermal AI, LoRaWAN/5G, gas detection, NFC access.

### Try it

```bash
go run ./cmd/relay-edge
# open http://127.0.0.1:18086/ui
# 1. Seed plant inventory
# 2. Pick a scenario (e.g. "Edge-AI fire detect")
# 3. Toggle "Publish into Relay" if gateway is wired
```

Or: `./scripts/smoke-firewater.sh` (local sim only, no Relay required).

### Scenarios (14)

| Scenario | What breaks | Example event |
|----------|-------------|---------------|
| `normal` | Healthy plant | — |
| `lowtank` | Tank level drops | `firewater.tank.low` |
| `lowpress` | Riser / jockey pressure low | `firewater.pressure.low` |
| `fire` | Demand flow, pump on | `firewater.demand.active` |
| `pumpfail` | Pump not delivering | `firewater.pump.fail` |
| `valve` | Valve closed / tamper | `firewater.valve.closed` |
| `leak` | Acoustic leak signature | `firewater.leak.acoustic` |
| `freeze` | Room temp low | `firewater.freeze.risk` |
| `hydrant` | Hydrant tamper | `firewater.hydrant.tamper` |
| `comms` | MQTT/cellular down | `edge.comms.down` |
| `vision` | Thermal AI score high | `edge.vision.fire` |
| `power` | Mains fail, genset | `edge.power.fail` |
| `gas` | LEL / CO exceedance | `edge.gas.alarm` |
| `plc` | PLC / FACP fault | `edge.control.fault` |

### Derived event types (20 + optional telemetry)

`firewater.tank.low` · `firewater.pressure.low` · `firewater.demand.active` · `firewater.pump.fail` · `firewater.valve.closed` · `firewater.flow.detected` · `firewater.freeze.risk` · `firewater.hydrant.tamper` · `firewater.pumproom.flood` · `firewater.diesel.low` · `firewater.leak.acoustic` · `firewater.pump.vibration` · `edge.vision.fire` · `edge.comms.down` · `edge.power.fail` · `edge.gas.alarm` · `edge.control.fault` · `edge.access.breach` · `edge.runtime.down` · `telemetry.sample` (when `telemetry_always: true`).

Action target: **`firewater-controller`**.

### Extra APIs

Interlocks (`/v1/firewater/act`), ISA-18.2 alarms (ack/shelve), Sparkplug B, Modbus holding map, NFPA 25 weekly test, topology graph, cause-and-effect matrix — see [API reference](API.md).

---

## Remote edge — distributed site NOC

Descriptive names for gear a remote-edge NOC tracks — Galleon compute, Starlink, SD-WAN, private 5G, UAV, vision, yard IoT. Not an Armada product.

### Try it

```bash
curl -fsS -X POST http://127.0.0.1:18086/v1/firewater/seed   # shared season first
# http://127.0.0.1:18086/ui/remote-edge.html
./scripts/smoke-remote-edge.sh
```

### Scenarios → events (8)

| Scenario | Event type |
|----------|------------|
| `nominal` | Healthy readings |
| `sat_down` | `remote-edge.link.starlink.degraded` |
| `offline` | `remote-edge.link.offline` |
| `gpu_hot` | `remote-edge.galleon.thermal` |
| `intrusion` | `remote-edge.vision.intrusion` |
| `flood` | `remote-edge.iot.flood` |
| `drone_patrol` | `remote-edge.uav.rtb` | Battery low (sim sets 15%) |
| `p5g_load` | High private 5G load (readings only) |

Action target: **`remote-edge-controller`**.

---

## Fleet — master edge catalog

One simulator covering **all edge classes**: robot/AMR, RTLS, wearables, energy, building, OT gateways, machine vision, water, environment, rail, agri, marine, life safety, radio, security.

### Try it

```bash
curl -fsS -X POST http://127.0.0.1:18086/v1/firewater/seed
# http://127.0.0.1:18086/ui/fleet.html
```

Filter by class chip, pick a scenario, watch readings and derived events update.

### Scenarios → events (8)

| Scenario | Event type | Suggested action |
|----------|------------|------------------|
| `nominal` | Healthy fleet | — |
| `blackout` | `fleet.power.island` | `bess.discharge` |
| `amr_lost` | `fleet.robot.lost` | `amr.relocalize` |
| `ot_storm` | `fleet.ot.ids` | `ot.segment` |
| `spill` | `fleet.env.exceedance` | `process.curtail` |
| `heatwave` | `fleet.dc.thermal` | `workload.shed` |
| `intrusion` | `fleet.access.fault` | `security.lockdown` |
| `flood` | Readings shift (no dedicated fleet event) | — |

Action target: **`fleet-controller`**.

---

## Publishing pipeline

```text
  Scenario/tick
       │
       ▼
  Derive(events[])     ← per-simulator logic (firewater / remoteedge / fleet)
       │
       ▼
  resolveEnrich + stampData   ← shared (season_fw_watch context)
       │
       ▼
  relay-pubsub topic   ← event type as topic name (or direct /v1/events)
       │
       ▼
  Relay Accept
```

Smoke locally without Relay: keep `"publish": false` — events appear in `/ui` SSE stream and `/v1/*/events` log only.

Full env and gateway wiring → [Configuration](CONFIGURATION.md) · [Working with Relay](RELAY.md).
