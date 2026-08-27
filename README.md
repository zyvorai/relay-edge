# Zyvor Relay Edge

**Season / site companion for [Zyvor Relay](https://github.com/zyvorai/relay)** on farm edge nodes.

Relay owns the durable **Accept → Notify → Ack → Act → Verify** loop.  
`relay-edge` owns **growing seasons, sites, and crop windows**, and publishes events *into* Relay (via Pub/Sub gateway or direct `/v1/events`).

```text
Agronomy / season calendar (relay-edge :18086)
        │  open / close / seasonal events
        ▼
Pub/Sub gateway (:18083)  ──or──  POST /v1/events
        │
        ▼
Zyvor Relay (:18080)  →  notify → ack → act → verify
```

Apache-2.0 · Module: `github.com/zyvorai/relay-edge`

## Why it exists

Relay policies match **event types + severity**. They do not model Kharif/Rabi calendars.  
`relay-edge` stores that domain and stamps every published event with `season_id`, `crop`, `site` so Relay timelines and SQL evidence stay season-aware.

## API

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Liveness |
| GET/POST | `/v1/seasons` | List / create |
| GET/PUT/DELETE | `/v1/seasons/{id}` | CRUD |
| POST | `/v1/seasons/{id}/open` | Status → `active` + `crop.advisory` into Relay |
| POST | `/v1/seasons/{id}/close` | Status → `closed` + advisory |
| POST | `/v1/seasons/{id}/events` | Publish typed farm event (must be `active`) |

Example critical event body:

```json
{
  "type": "irrigation.required",
  "severity": "critical",
  "command": "irrigation.start",
  "zone": "A4",
  "data": { "duration_minutes": 15 }
}
```

## Env

| Var | Default | Meaning |
|-----|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen |
| `EDGE_DATA_DIR` | `./data` | `seasons.json` store |
| `GATEWAY_BASE_URL` | `http://127.0.0.1:18083` | fasal-pubsub-gateway / relay-pubsub REST |
| `RELAY_BASE_URL` | `https://127.0.0.1:18080` | Direct fallback |
| `RELAY_AUTH_TOKEN` | — | JWT for direct `/v1/events` |
| `RELAY_TLS_INSECURE` | `1` | Lab self-signed |
| `FASAL_GCP_PROJECT` | `fasal-onprem` | Gateway project id |

## Local

```bash
go run ./cmd/relay-edge
EDGE=http://127.0.0.1:18086 ./scripts/smoke.sh
```

## Lab deploy (`212.8.248.187`)

```bash
# Optional: export RELAY_AUTH_TOKEN from OIDC (scripts/lib/auth.sh in Relay repo)
./scripts/deploy-remote.sh 212.8.248.187 sus
EDGE=http://212.8.248.187:18086 ./scripts/smoke.sh
```

Then open Relay console → Events — you should see season advisories and irrigation with `season_id` in the payload.

## Related

| Repo | Role |
|------|------|
| [zyvorai/relay](https://github.com/zyvorai/relay) | Control plane |
| [zyvorai/relay-pubsub](https://github.com/zyvorai/relay-pubsub) | Google Pub/Sub wire |
| Relay `examples/fasal-pubsub-gateway` | Go reference gateway on lab `:18083` |
