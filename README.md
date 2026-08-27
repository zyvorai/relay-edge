# Zyvor Relay Edge

**Farm-domain companion for [Zyvor Relay](https://github.com/zyvorai/relay)** on edge nodes.

Relay owns the durable **Accept → Notify → Ack → Act → Verify** loop.  
`relay-edge` owns **seasons, sites/zones, devices, contacts, telemetry probes, and growth stages**, and publishes enriched events *into* Relay (via Pub/Sub gateway or direct `/v1/events`).

```text
relay-edge (:18086)
  seasons · sites/zones · devices · contacts · probes · stages
        │  stamp season_id / site_id / zone / fasal_device_id / recipients / verification_probe
        ▼
Pub/Sub gateway (:18083)  ──or──  POST /v1/events
        │
        ▼
Zyvor Relay (:18080)  →  notify → ack → act → verify
```

Apache-2.0 · Module: `github.com/zyvorai/relay-edge`

## Why it exists

Relay policies match **event types + severity**. They do not model Kharif/Rabi calendars, plot maps, device inventory, or farmer contacts.  
`relay-edge` stores that domain and stamps every published event so Relay timelines and SQL evidence stay farm-aware.

## API

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Liveness (+ module list) |
| GET/POST | `/v1/sites` | List / create sites |
| GET/PUT/DELETE | `/v1/sites/{id}` | Site CRUD |
| PUT | `/v1/sites/{id}/routing` | Role → `contact_id` map |
| GET/POST | `/v1/sites/{id}/zones` | Zones under a site |
| GET/PUT/DELETE | `/v1/zones/{id}` | Zone CRUD |
| GET/PUT/DELETE | `/v1/zones/{id}/telemetry` | Verification probe registry |
| GET/POST | `/v1/devices` | Device inventory (`?zone_id=`) |
| GET/PUT/DELETE | `/v1/devices/{id}` | Device CRUD |
| GET/POST | `/v1/contacts` | Notify contacts |
| GET/PUT/DELETE | `/v1/contacts/{id}` | Contact CRUD |
| GET/POST | `/v1/seasons` | Seasons (prefer `site_id`) |
| GET/PUT/DELETE | `/v1/seasons/{id}` | Season CRUD |
| POST | `/v1/seasons/{id}/open` | → `active` + `crop.advisory` |
| POST | `/v1/seasons/{id}/close` | → `closed` + advisory |
| POST | `/v1/seasons/{id}/stage` | Growth stage + `crop.advisory` |
| POST | `/v1/seasons/{id}/advisories` | `crop`/`spray`/`frost`/`weather`/`pest` |
| POST | `/v1/seasons/{id}/events` | Critical farm event (must be `active`) |

Critical event body example:

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

Publish stamps `season_id`, `site_id`, `zone`/`zone_id`, `fasal_device_id`, notify recipients from site routing, and `verification_probe` from the zone when set.

## Env

| Var | Default | Meaning |
|-----|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen |
| `EDGE_DATA_DIR` | `./data` | JSON stores (`seasons.json`, `sites.json`, …) |
| `GATEWAY_BASE_URL` | `http://127.0.0.1:18083` | fasal-pubsub-gateway / relay-pubsub REST |
| `RELAY_BASE_URL` | `https://127.0.0.1:18080` | Direct fallback |
| `RELAY_AUTH_TOKEN` | — | JWT for direct `/v1/events` |
| `RELAY_TLS_INSECURE` | `1` | Lab self-signed |
| `FASAL_GCP_PROJECT` | `fasal-onprem` | Gateway project id |

## Local

```bash
go test ./...
go run ./cmd/relay-edge
EDGE=http://127.0.0.1:18086 ./scripts/smoke.sh
```

## Lab deploy (`212.8.248.187`)

```bash
./scripts/deploy-remote.sh 212.8.248.187 sus
EDGE=http://212.8.248.187:18086 ./scripts/smoke.sh
```

## Related

| Repo | Role |
|------|------|
| [zyvorai/relay](https://github.com/zyvorai/relay) | Control plane |
| [zyvorai/relay-pubsub](https://github.com/zyvorai/relay-pubsub) | Google Pub/Sub wire |
| Relay `examples/fasal-pubsub-gateway` | Go reference gateway on lab `:18083` |
| Relay `examples/fasaljet-adapter` | Act + telemetry probe stub |
