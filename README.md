# Zyvor Relay Edge

**Farm-domain companion for [Zyvor Relay](https://github.com/zyvorai/relay).**

Relay runs the durable **Accept → Notify → Ack → Act → Verify** loop.  
`relay-edge` owns the farm: seasons, sites, zones, devices, contacts, and telemetry probes — then stamps every event so Relay policies stay farm-aware without embedding crop calendars or plot maps.

```text
                    ┌─────────────────────────────────────┐
                    │           relay-edge :18086         │
                    │  seasons · sites · zones · devices  │
                    │  contacts · probes · growth stages  │
                    └─────────────────┬───────────────────┘
                                      │ stamp + publish
                    ┌─────────────────▼───────────────────┐
                    │  Pub/Sub gateway :18083  (preferred)│
                    │           — or —                    │
                    │  POST Relay /v1/events   (fallback) │
                    └─────────────────┬───────────────────┘
                                      │
                    ┌─────────────────▼───────────────────┐
                    │         Zyvor Relay :18080          │
                    │   notify → ack → act → verify       │
                    └─────────────────────────────────────┘
```

Apache-2.0 · `github.com/zyvorai/relay-edge`

---

## How it works

### 1. Domain lives on the edge

JSON stores under `EDGE_DATA_DIR` hold farm state:

| Store | What it models |
|-------|----------------|
| **Sites** | Farms / orchards + role → contact routing |
| **Zones** | Plots / blocks (e.g. `A4`) + optional verification probes |
| **Devices** | Valves, sensors, jets (`external_id` → `fasal_device_id`) |
| **Contacts** | Farmers / operators (FCM, SMS, email) |
| **Seasons** | Crop calendar (`planned` → `active` → `closed`) + growth stage |

Relay never stores this. Edge does — so policies can match event type + severity while timelines stay labeled with season, site, and zone.

### 2. Lifecycle drives publishes

Mutations that matter to the control plane publish into Relay:

| Action | Event |
|--------|--------|
| Open / close season | `crop.advisory` |
| Set growth stage | `crop.advisory` |
| Post advisory | `crop` / `spray` / `frost` / `weather` / `pest` … |
| Critical farm event (season must be `active`) | e.g. `irrigation.required` |

### 3. Every event is stamped

Before publish, edge enriches the payload from domain stores:

- `season_id`, `site_id`, `zone` / `zone_id`
- `fasal_device_id` from the device inventory
- notify **recipients** from site role routing
- `verification_probe` from the zone (so Relay can verify acts)

That stamp is what makes Relay evidence farm-aware.

### 4. Two publish paths

1. **Gateway (preferred)** — when `GATEWAY_BASE_URL` is set, posts to the Pub/Sub-compatible REST API (`…/topics/{eventType}:publish`). Topic name = event type.
2. **Direct Relay** — otherwise `POST {RELAY_BASE_URL}/v1/events`.

Auth tokens (`GATEWAY_AUTH_TOKEN`, `RELAY_AUTH_TOKEN`) are optional and forwarded as Bearer when present.

### 5. Typical critical path

```text
POST /v1/seasons/{id}/events
  { type, severity, command, zone_id, device_id, data }
        │
        ▼
  resolve season → site → zone → device → contacts
  stamp recipients + verification_probe
        │
        ▼
  publish → Relay Accept
        │
        ▼
  Notify farmer → Ack → Act (irrigation.start) → Verify via probe
```

---

## Quick start

```bash
go test ./...
go run ./cmd/relay-edge
./scripts/smoke.sh
```

Smoke walks site → zone → contact → device → season → stage → advisory → irrigation against a running edge (`EDGE` defaults to `http://127.0.0.1:18086`).

---

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen address |
| `EDGE_DATA_DIR` | `./data` | JSON stores |
| `GATEWAY_BASE_URL` | `http://127.0.0.1:18083` | Pub/Sub gateway (preferred publish path) |
| `GATEWAY_AUTH_TOKEN` | — | Bearer for gateway when auth is on |
| `RELAY_BASE_URL` | `https://127.0.0.1:18080` | Direct `/v1/events` fallback |
| `RELAY_AUTH_TOKEN` | — | JWT for direct Relay |
| `RELAY_TLS_INSECURE` | `1` | Allow self-signed Relay TLS (lab) |
| `FASAL_GCP_PROJECT` | `fasal-onprem` | Gateway project id |

---

## API

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Liveness (+ modules) |
| GET/POST | `/v1/sites` | List / create sites |
| GET/PUT/DELETE | `/v1/sites/{id}` | Site CRUD |
| PUT | `/v1/sites/{id}/routing` | Role → `contact_id` map |
| GET/POST | `/v1/sites/{id}/zones` | Zones under a site |
| GET/PUT/DELETE | `/v1/zones/{id}` | Zone CRUD |
| GET/PUT/DELETE | `/v1/zones/{id}/telemetry` | Verification probe |
| GET/POST | `/v1/devices` | Device inventory (`?zone_id=`) |
| GET/PUT/DELETE | `/v1/devices/{id}` | Device CRUD |
| GET/POST | `/v1/contacts` | Notify contacts |
| GET/PUT/DELETE | `/v1/contacts/{id}` | Contact CRUD |
| GET/POST | `/v1/seasons` | Seasons (prefer `site_id`) |
| GET/PUT/DELETE | `/v1/seasons/{id}` | Season CRUD |
| POST | `/v1/seasons/{id}/open` | → `active` + advisory |
| POST | `/v1/seasons/{id}/close` | → `closed` + advisory |
| POST | `/v1/seasons/{id}/stage` | Growth stage + advisory |
| POST | `/v1/seasons/{id}/advisories` | Typed advisory publish |
| POST | `/v1/seasons/{id}/events` | Critical farm event (must be `active`) |

### Critical event example

```json
{
  "type": "irrigation.required",
  "severity": "critical",
  "command": "irrigation.start",
  "zone_id": "zone_…",
  "device_id": "dev_…",
  "data": { "duration_minutes": 15 }
}
```

---

## Remote deploy

Build a Linux binary and run it on a host (requires SSH). **Host is required** — no baked-in lab address.

```bash
# Optional: RELAY_AUTH_TOKEN and/or GATEWAY_AUTH_TOKEN
# (or drop JWTs at /tmp/lab-relay.jwt and /tmp/lab-gateway.token)
./scripts/deploy-remote.sh <HOST> [USER]

EDGE=http://<HOST>:18086 ./scripts/smoke.sh
```

Deploys to `~/.deployments/zyvor-relay-edge` (isolated from other products). On the remote, edge talks to local Relay (`:18080`) and gateway (`:18083`).

---

## Related

| Repo | Role |
|------|------|
| [zyvorai/relay](https://github.com/zyvorai/relay) | Control plane — Accept → Notify → Ack → Act → Verify |
| [zyvorai/relay-pubsub](https://github.com/zyvorai/relay-pubsub) | Google Pub/Sub wire |
| Relay `examples/fasal-pubsub-gateway` | Reference gateway on `:18083` |
| Relay `examples/fasaljet-adapter` | Act + telemetry probe stub |
