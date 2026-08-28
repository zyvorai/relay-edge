# API reference

Base URL: `http://127.0.0.1:18086` (or `https://…` when `EDGE_TLS=1`).

← [Docs hub](README.md)

← [Docs hub](README.md)

---

All paths return JSON unless noted (SSE for `/v1/firewater/stream`).

## Health

| Method | Path | Response |
|--------|------|----------|
| GET | `/healthz` | `{ "status": "ok", "modules": [...] }` |

Modules include `seasons`, `sites`, `zones`, `devices`, `contacts`, `telemetry`, `stages`, `firewater`, `remote-edge`, `fleet`, `ui`.

---

## Farm domain

### Sites & zones

| Method | Path | Notes |
|--------|------|-------|
| GET/POST | `/v1/sites` | List / create |
| GET/PUT/DELETE | `/v1/sites/{id}` | CRUD |
| PUT | `/v1/sites/{id}/routing` | `{ "routing": { "farmer": "contact_id", … } }` |
| GET/POST | `/v1/sites/{id}/zones` | Zones under site |
| GET/PUT/DELETE | `/v1/zones/{id}` | Zone CRUD |
| GET/PUT/DELETE | `/v1/zones/{id}/telemetry` | Verification probe for Act |

### Devices & contacts

| Method | Path | Notes |
|--------|------|-------|
| GET/POST | `/v1/devices` | `?zone_id=` filter |
| GET/PUT/DELETE | `/v1/devices/{id}` | Includes `external_id`, `commands` |
| GET/POST | `/v1/contacts` | FCM, SMS, email |
| GET/PUT/DELETE | `/v1/contacts/{id}` | |

### Seasons

| Method | Path | Notes |
|--------|------|-------|
| GET/POST | `/v1/seasons` | Prefer `site_id` |
| GET/PUT/DELETE | `/v1/seasons/{id}` | |
| POST | `/v1/seasons/{id}/open` | → `active`, publishes advisory |
| POST | `/v1/seasons/{id}/close` | → `closed` |
| POST | `/v1/seasons/{id}/stage` | `{ "stage": "vegetative" }` |
| POST | `/v1/seasons/{id}/advisories` | Typed advisory publish |
| POST | `/v1/seasons/{id}/events` | Critical event — season must be `active` |

### Critical event body

```json
{
  "type": "irrigation.required",
  "severity": "critical",
  "command": "irrigation.start",
  "zone_id": "zone_abc",
  "device_id": "dev_valve",
  "data": { "duration_minutes": 15 }
}
```

---

## Firewater

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/v1/firewater/catalog` | Point definitions |
| GET | `/v1/firewater/snapshot` | Live readings |
| GET | `/v1/firewater/stream` | SSE ticks + events |
| POST | `/v1/firewater/seed` | Idempotent plant inventory |
| POST | `/v1/firewater/config` | `{ "publish", "interval_ms", "scenario", … }` |
| POST | `/v1/firewater/start` / `stop` / `tick` / `scenario` | Simulator control |
| GET | `/v1/firewater/ready` | System ready + why not |
| GET | `/v1/firewater/topology` | Pipe graph |
| GET | `/v1/firewater/matrix` | Cause-and-effect rules |
| POST | `/v1/firewater/act` | `{ "command": "pump.start" }` |
| GET | `/v1/firewater/verify?command=…` | Post-act probe |
| GET | `/v1/firewater/alarms` | Standing alarms |
| POST | `/v1/firewater/alarms/{id}/ack` | Ack alarm |
| POST | `/v1/firewater/alarms/{id}/shelve` | `{ "minutes": 15 }` |
| GET | `/v1/firewater/sparkplug` | Sparkplug B NDATA |
| GET | `/v1/firewater/modbus` | Holding registers from 40001 |
| POST | `/v1/firewater/weekly-test` | NFPA 25 evidence |

---

## Remote edge

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/v1/remote-edge/catalog` | Asset catalog |
| GET | `/v1/remote-edge/snapshot` | Readings + link mode + publish state |
| GET | `/v1/remote-edge/config` | Current `{ publish, interval_ms }` |
| GET | `/v1/remote-edge/events` | Recent derived events log |
| GET | `/v1/remote-edge/stream` | SSE live ticks + events |
| POST | `/v1/remote-edge/config` | `{ "publish": true, "interval_ms": 2000 }` |
| POST | `/v1/remote-edge/tick` / `start` / `stop` / `scenario` | Simulator control |

Scenarios: `nominal`, `sat_down`, `offline`, `gpu_hot`, `drone_patrol`, `intrusion`, `flood`, `p5g_load`.

---

## Fleet

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/v1/fleet/catalog` | Classes + device list |
| GET | `/v1/fleet/snapshot` | Readings by class + publish state |
| GET | `/v1/fleet/config` | Current `{ publish, interval_ms }` |
| GET | `/v1/fleet/events` | Recent derived events log |
| GET | `/v1/fleet/stream` | SSE live ticks + events |
| POST | `/v1/fleet/config` | `{ "publish": true, "interval_ms": 2000 }` |
| POST | `/v1/fleet/tick` / `start` / `stop` / `scenario` | Simulator control |

Scenarios: `nominal`, `blackout`, `intrusion`, `spill`, `amr_lost`, `ot_storm`, `heatwave`, `flood`.

---

## Web UIs

| Path | Description |
|------|-------------|
| GET `/ui` | Fire-water control room |
| GET `/ui/remote-edge.html` | Remote edge fleet |
| GET `/ui/fleet.html` | Master edge catalog |

Static assets are embedded in the binary (`web/embed.go`).
