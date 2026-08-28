# Simulators

Three companion simulators ship inside relay-edge — each with a web UI, REST API, and optional Relay publish.

← [Docs hub](README.md)

---

## Overview

| Simulator | UI | Points | Publish config |
|-----------|-----|--------|----------------|
| **Firewater** | [/ui](../web/index.html) | 47 | `POST /v1/firewater/config` |
| **Atlas** | [/ui/atlas.html](../web/atlas.html) | ~20 readings | `POST /v1/atlas/config` |
| **Fleet** | [/ui/fleet.html](../web/fleet.html) | 60+ | `POST /v1/fleet/config` |

Enable publish on any simulator:

```json
{ "publish": true }
```

Requires `POST /v1/firewater/seed` first (creates shared industrial season/site/zone).

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

### Scenarios

| Scenario | What breaks | Example event |
|----------|-------------|---------------|
| `lowtank` | Tank level drops | `firewater.tank.low` |
| `fire` | Demand flow, pump on | `firewater.demand.active` |
| `comms` | MQTT/cellular down | `edge.comms.down` |
| `vision` | Thermal AI score high | `edge.vision.fire` |
| `gas` | LEL / CO exceedance | `edge.gas.alarm` |
| `pumpfail` | Pump not delivering | `firewater.pump.fail` |
| `valve` | Valve closed / tamper | `firewater.valve.closed` |
| `freeze` | Room temp low | `firewater.freeze.risk` |

### Extra APIs

Interlocks (`/v1/firewater/act`), ISA-18.2 alarms, Sparkplug B, Modbus holding map, NFPA 25 weekly test — see [API reference](API.md).

---

## Atlas — remote edge fleet

Descriptive names for the **kinds of gear** a remote-edge NOC tracks — Galleon compute, Starlink, SD-WAN, private 5G, UAV, vision, yard IoT. Not an Armada product.

### Try it

```bash
# http://127.0.0.1:18086/ui/atlas.html
./scripts/smoke-atlas.sh
```

### Scenarios → events

| Scenario | Event type |
|----------|------------|
| `sat_down` | `atlas.link.starlink.degraded` |
| `offline` | `atlas.link.offline` |
| `gpu_hot` | `atlas.galleon.thermal` |
| `intrusion` | `atlas.vision.intrusion` |
| `flood` | `atlas.iot.flood` |
| `drone_patrol` | `atlas.uav.rtb` (when battery low) |

---

## Fleet — master edge catalog

One simulator covering **all edge classes**: robot/AMR, RTLS, wearables, energy, building, OT gateways, machine vision, water, environment, rail, agri, marine, life safety, radio, security.

### Try it

```bash
# http://127.0.0.1:18086/ui/fleet.html
```

Filter by class chip, pick a scenario, watch readings and derived events update.

### Scenarios → events

| Scenario | Event type | Suggested action |
|----------|------------|------------------|
| `blackout` | `fleet.power.island` | `bess.discharge` |
| `amr_lost` | `fleet.robot.lost` | `amr.relocalize` |
| `ot_storm` | `fleet.ot.ids` | `ot.segment` |
| `spill` | `fleet.env.exceedance` | `process.curtail` |
| `heatwave` | `fleet.dc.thermal` | `workload.shed` |
| `intrusion` | `fleet.access.fault` | `security.lockdown` |

---

## Publishing pipeline

```text
  Scenario/tick
       │
       ▼
  Derive(events[])     ← per-simulator logic
       │
       ▼
  stamp + publish      ← shared sim_publish.go
       │
       ▼
  relay-pubsub topic   ← event type as topic name
       │
       ▼
  Relay Accept
```

Smoke locally without Relay: keep `"publish": false` — events appear in `/ui` SSE stream and `/v1/firewater/events` log only.
