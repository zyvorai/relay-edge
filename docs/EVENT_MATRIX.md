# Event matrix

The integration gate: prove every event family reaches Relay through relay-pubsub.

← [Docs hub](README.md) · Requires [relay-pubsub](https://github.com/zyvorai/relay-pubsub) with `RELAY_BACKEND=relay-events`

---

## Run it

```bash
BASE=https://<relay-host>:8443 \
GATEWAY=https://<gateway-host>:8081 \
EDGE=http://<edge-host>:18086 \
  ./scripts/e2e-events-matrix.sh
```

The script prints a pass/fail table. Non-zero exit if any row fails.

**Direct Relay (no pubsub):** use [`scripts/e2e-direct-relay.sh`](../scripts/e2e-direct-relay.sh) — expanded scenario matrix via `POST /v1/events`. See [config/lab-direct.env.example](../config/lab-direct.env.example).

**Latest verification:** [TEST_RESULTS.md](TEST_RESULTS.md) — 2026-08-28, gateway + direct PASS.

---

## What it checks

| Section | Source | Gate |
|---------|--------|------|
| **A. Farm** | Gateway REST (fasal-catalog-smoke) | 10× Accept; 5× critical Act |
| **B. Firewater** | Edge scenarios + `publish:true` | New event in Relay |
| **C. Remote edge** | Edge scenarios + `publish:true` | New event in Relay |
| **D. Fleet** | Edge scenarios + `publish:true` | New event in Relay |

Smoke scripts alone (`smoke.sh`, `smoke-firewater.sh`) **do not** replace this matrix for pubsub integration.

---

## A. Farm catalog

| Event type | Severity | Command | Expected policy |
|------------|----------|---------|-----------------|
| `irrigation.required` | critical | `irrigation.start` | `pol_critical_farm` + Act |
| `soil.moisture.critical` | critical | `irrigation.start` | Act |
| `fertigation.required` | critical | `fertigation.start` | Act |
| `disease.risk.critical` | critical | `inspection.create` | Act |
| `device.control.required` | critical | `pump.start` | Act |
| `crop.advisory` | info | — | `pol_advisory`, notify only |
| `weather.advisory` | info | — | notify only |
| `spray.advisory` | info | — | notify only |
| `frost.alert` | info | — | notify only |
| `pest.advisory` | info | — | notify only |

**Act wiring checklist**

1. `RELAY_ACTION_TARGETS=farm-controller=https://127.0.0.1:8081/v1/actions` on Relay
2. `PUBSUB_TLS_SAN` includes `127.0.0.1` (regenerate cert if needed)
3. Relay outbound: `RELAY_TLS_INSECURE=1` or trust gateway CA
4. Successful Act shows `provider_id` prefix `rpg_`

---

## B. Firewater / edge

Prerequisite: `POST /v1/firewater/seed` + `POST /v1/firewater/config` `{"publish":true}`

Remote-edge and fleet sections also require firewater seed (shared `season_fw_watch`).

| Scenario | Event type |
|----------|------------|
| `lowtank` | `firewater.tank.low` |
| `fire` | `firewater.demand.active` |
| `comms` | `edge.comms.down` |
| `vision` | `edge.vision.fire` |
| `gas` | `edge.gas.alarm` |

Full catalog: all `firewater.*`, `edge.*`, `telemetry.sample` — see [Simulators](SIMULATORS.md).

---

## C. Remote edge

Prerequisite: `POST /v1/remote-edge/config` `{"publish": true}`

| Scenario | Event type |
|----------|------------|
| `sat_down` | `remote-edge.link.starlink.degraded` |
| `offline` | `remote-edge.link.offline` |
| `gpu_hot` | `remote-edge.galleon.thermal` |
| `intrusion` | `remote-edge.vision.intrusion` |
| `flood` | `remote-edge.iot.flood` |
| `drone_patrol` | `remote-edge.uav.rtb` |

Also derivable: `remote-edge.uav.rtb` (`drone_patrol` scenario)

Prerequisite for gateway matrix includes `drone_patrol` (6 remote-edge event types).

---

## D. Fleet

Prerequisite: `POST /v1/fleet/config` `{"publish": true}`

| Scenario | Event type |
|----------|------------|
| `blackout` | `fleet.power.island` |
| `amr_lost` | `fleet.robot.lost` |
| `ot_storm` | `fleet.ot.ids` |
| `spill` | `fleet.env.exceedance` |
| `heatwave` | `fleet.dc.thermal` |
| `intrusion` | `fleet.access.fault` |

---

## Status snapshot

Typical lab results after wiring Accept paths:

| Family | Accept | Act |
|--------|--------|-----|
| Farm advisory (5) | ✅ | n/a |
| Farm critical (5) | ✅ | ⚠️ needs Relay→gateway TLS trust |
| Firewater / edge | ✅ | — |
| Remote edge | ✅ | — |
| Fleet | ✅ | — |
| k8s stack e2e | ✅ | relay-events publish verified |

---

## relay-pubsub pre-registers 40 topics

At startup, relay-pubsub registers farm + edge + remote-edge + fleet catalogs for admin visibility. Publish works for any topic name regardless — see [relay-pubsub docs](https://github.com/zyvorai/relay-pubsub/blob/main/docs/RELAY_EVENTS_BACKEND.md).
