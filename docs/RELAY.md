# Working with Relay

How relay-edge publishes into [Zyvor Relay](https://github.com/zyvorai/relay) — with or without relay-pubsub in the middle.

← [Docs hub](README.md) · See also [Concepts](CONCEPTS.md) · [Event matrix](EVENT_MATRIX.md)

---

## What relay-edge does for Relay

Relay owns the durable control loop:

```text
Accept → Notify → Ack → Act → Verify
```

relay-edge owns everything **before** Accept:

| relay-edge | Relay |
|------------|-------|
| Sites, zones, devices, seasons | Event log + policies |
| Stamp season/site/zone/device context into `data` | Match on `type` + `severity` |
| Simulators (firewater, remote-edge, fleet) | Notify recipients, run acts |
| Web control rooms | Verify via telemetry probe |

relay-edge **never** runs the notify/ack/act loop. It produces **policy-ready events** and POSTs them into Relay.

---

## Two ways to reach Relay

```text
                    ┌─────────────────────┐
                    │     relay-edge      │
                    │  stamp + publish    │
                    └──────────┬──────────┘
                               │
           ┌───────────────────┴───────────────────┐
           │                                       │
           ▼                                       ▼
  GATEWAY_BASE_URL set                   GATEWAY_BASE_URL empty
  (preferred in production)              (direct / dev / minimal stack)
           │                                       │
           ▼                                       ▼
  relay-pubsub                         POST {RELAY_BASE_URL}/v1/events
  POST …/topics/{type}:publish                    │
           │                                       │
           └───────────────────┬───────────────────┘
                               ▼
                         Zyvor Relay
                         Accept → …
```

Implementation: [`internal/relaypub/client.go`](../internal/relaypub/client.go) — `PublishEventType()` picks the path automatically.

| Path | When to use | Env vars |
|------|-------------|----------|
| **Via relay-pubsub** | Google Pub/Sub clients, multi-tenant gateway, k8s stack | `GATEWAY_BASE_URL`, optional `GATEWAY_AUTH_TOKEN`, `FASAL_GCP_PROJECT` |
| **Direct to Relay** | Edge-only deploy, debugging, no gateway process | Unset `GATEWAY_BASE_URL`; set `RELAY_BASE_URL` + `RELAY_AUTH_TOKEN` |

Both paths send the **same logical event** — type, severity, source, idempotency key, stamped `data`.

---

## Direct mode — configuration

Run edge with gateway disabled:

```bash
export GATEWAY_BASE_URL=          # empty = direct path
export RELAY_BASE_URL=https://127.0.0.1:8443
export RELAY_AUTH_TOKEN=<jwt-from-relay>
export RELAY_TLS_INSECURE=1       # if Relay uses self-signed TLS
go run ./cmd/relay-edge
```

On startup, edge logs which URLs it uses:

```text
relay-edge listening on http://:18086 (data=./data gateway= relay=https://127.0.0.1:8443 tls=false)
```

When `gateway=` is empty, every publish goes straight to Relay.

### systemd / deploy

In `scripts/deploy-remote.sh` the default is gateway mode (`GATEWAY_BASE_URL=https://127.0.0.1:8081`). For direct-only, override on the host:

```bash
# In relay-edge env file on the host:
GATEWAY_BASE_URL=
RELAY_BASE_URL=https://127.0.0.1:8443
RELAY_AUTH_TOKEN=<jwt>
RELAY_TLS_INSECURE=1
```

---

## Direct wire contract

Edge POSTs to Relay's native events API:

```http
POST /v1/events
Authorization: Bearer <RELAY_AUTH_TOKEN>
Content-Type: application/json
```

Body (produced by `publishDirect`):

```json
{
  "type": "irrigation.required",
  "severity": "critical",
  "source": "FW Plant / Watch Season / grape / Zone FW-A",
  "idempotency_key": "edge/season/season_abc/open/1735123456",
  "data": {
    "season_id": "season_fw_watch",
    "season_name": "Industrial watch",
    "site_id": "site_fw_plant",
    "site": "Fire-water plant",
    "zone_id": "zone_fw_process_a",
    "zone": "FW-A",
    "device_id": "dev_fw_pump_main",
    "fasal_device_id": "fw-pump-01",
    "recipient": "fcm-token-or-demo",
    "verification_probe": {
      "url": "http://127.0.0.1:18086/v1/firewater/verify?command=pump.start",
      "method": "GET",
      "json_path": "$.ok",
      "expect": "true"
    },
    "recommended_action": {
      "target": "firewater-controller",
      "command": "pump.start",
      "payload": { "zone": "FW-A", "zone_id": "zone_fw_process_a", "device_id": "dev_fw_pump_main", "season_id": "season_fw_watch" }
    }
  }
}
```

Success response (edge stores `event.id` on seasons when applicable):

```json
{
  "event": { "id": "evt_…", "type": "irrigation.required", "status": "accepted" }
}
```

The `publish` field in API responses shows which path was used:

```json
{ "publish": { "path": "relay", "event_id": "evt_…" } }
```

Gateway path returns `"path": "gateway"` instead.

---

## Gateway wire contract (for comparison)

When `GATEWAY_BASE_URL` is set, edge calls Google Pub/Sub-compatible REST:

```http
POST /v1/projects/fasal-onprem/topics/irrigation.required:publish
Authorization: Bearer <GATEWAY_AUTH_TOKEN>   # optional
```

Body:

```json
{
  "messages": [{
    "data": "<base64-json-of-stamped-data>",
    "attributes": {
      "severity": "critical",
      "source": "FW Plant / Watch Season / …",
      "idempotency_key": "edge/firewater/…"
    }
  }]
}
```

relay-pubsub (`RELAY_BACKEND=relay-events`) decodes this and forwards the same fields to `POST /v1/events` on Relay.

Topic name **is** the event type. Project defaults to `FASAL_GCP_PROJECT` (`fasal-onprem`).

---

## The stamp — what edge adds before Relay sees the event

Every publish (farm API or simulator) runs through the same enrichment pipeline in [`internal/httpapi/server.go`](../internal/httpapi/server.go):

```text
season → site → zone → device → contact (from site routing)
                              → verification_probe (from zone telemetry)
```

Stamped fields in `data`:

| Field | Source |
|-------|--------|
| `season_id`, `season_name`, `crop`, `stage` | Season store |
| `site_id`, `site` | Site store |
| `zone_id`, `zone` | Zone store |
| `device_id`, `fasal_device_id` | Device store |
| `recipient`, `sms_recipient`, `email_recipient` | Contact via site routing |
| `verification_probe` | Zone telemetry config (for Act → Verify) |
| `recommended_action` | Simulator critical events (target + command) |
| `sim_domain` | `firewater`, `remote-edge`, or `fleet` for simulator events |

Farm critical events (`POST /v1/seasons/{id}/events`) require the season to be **`active`**.

Simulators require **`POST /v1/firewater/seed`** first — it creates the shared industrial season/site/zone/devices all simulators reuse.

---

## End-to-end lifecycle (direct or gateway)

```text
  ┌──────────────┐
  │  User / UI   │  Seed → scenario → Publish toggle
  │  or smoke.sh │
  └──────┬───────┘
         │ HTTP to relay-edge
         ▼
  ┌──────────────┐
  │ relay-edge   │  resolveEnrich → stampData
  └──────┬───────┘
         │
         ├── direct ──► POST Relay /v1/events
         │
         └── gateway ► POST pubsub …/topics/{type}:publish
                              │
                              ▼
                         POST Relay /v1/events
         │
         ▼
  ┌──────────────┐
  │    Relay     │  Accept (policy match on type + severity)
  └──────┬───────┘
         │ Notify (FCM/SMS/email from stamped recipients)
         ▼
       Ack (human or auto)
         │
         ▼
       Act (if critical + recommended_action)
         │  Relay POST /v1/actions → relay-pubsub Action Gateway
         ▼
       Verify (probe in stamped data)
```

### Act wiring (when using relay-pubsub)

For critical events that should **Act**, Relay must know where to send actions. On the Relay process:

```bash
RELAY_ACTION_TARGETS=farm-controller=https://127.0.0.1:8081/v1/actions,\
firewater-controller=https://127.0.0.1:8081/v1/actions,\
remote-edge-controller=https://127.0.0.1:8081/v1/actions,\
fleet-controller=https://127.0.0.1:8081/v1/actions
RELAY_TLS_INSECURE=1   # required when pubsub uses self-signed HTTPS
```

Lab helper (rewrites targets, restarts Relay, optional new binary):

```bash
RELAY_BIN=/path/to/linux-amd64-relay ./scripts/lab-wire-relay-act.sh <HOST>
# then re-sync relay-pubsub RELAY_AUTH_TOKEN and ./scripts/e2e-stack.sh
```

Direct mode still benefits from relay-pubsub for the **inbound action** path — edge only handles **outbound publish**. A minimal direct-only lab can run edge → Relay without Act unless you also run pubsub for `/v1/actions`.

---

## What publishes, and when

| Trigger | Event examples | Publish path |
|---------|----------------|--------------|
| `POST /v1/seasons/{id}/open` | `crop.advisory` | Always (if Relay reachable) |
| `POST /v1/seasons/{id}/events` | `irrigation.required`, … | Always (season must be active) |
| `POST /v1/seasons/{id}/advisories` | `weather.advisory`, … | Always |
| Firewater UI / API | `firewater.tank.low`, `edge.comms.down`, … | When `publish: true` in config |
| Remote-edge UI / API | `remote-edge.link.offline`, … | When `publish: true` in config |
| Fleet UI / API | `fleet.power.island`, … | When `publish: true` in config |

Local simulation without Relay: leave `publish: false` — events appear in the UI SSE stream only.

---

## Try direct mode locally

**1. Start Relay** (from the relay repo — port and JWT vary by your install):

```bash
# Example lab defaults:
# Relay HTTPS :8443, JWT in /tmp/lab-relay.jwt
```

**2. Start edge in direct mode:**

```bash
export GATEWAY_BASE_URL=
export RELAY_BASE_URL=https://127.0.0.1:8443
export RELAY_AUTH_TOKEN="$(cat /tmp/lab-relay.jwt)"
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
```

**3. Full direct matrix** (farm + 13 firewater + 6 remote-edge + 6 fleet scenarios):

```bash
cp config/lab-direct.env.example config/lab-direct.env
# BASE, EDGE, RELAY_AUTH_TOKEN — relay-edge must have GATEWAY_BASE_URL empty

set -a && source config/lab-direct.env && set +a
./scripts/stack-probe.sh --direct
./scripts/e2e-direct-relay.sh
# or: ./scripts/e2e-direct-stack.sh
```

Deploy relay-edge in direct mode on a remote host:

```bash
RELAY_EDGE_DIRECT=1 RELAY_AUTH_TOKEN=<jwt> ./scripts/deploy-remote.sh <HOST> [USER]
```

**4. Farm smoke** (creates domain + publishes through edge → Relay direct):

```bash
./scripts/smoke.sh
```

Check the last response — `publish.path` should be `"relay"` and include `event_id`.

**5. Simulator via UI:**

```bash
# open http://127.0.0.1:18086/ui
# Seed → Publish into Relay → scenario "Low tank level"
```

**6. Confirm in Relay:**

```bash
curl -k -H "Authorization: Bearer $(cat /tmp/lab-relay.jwt)" \
  "https://127.0.0.1:8443/v1/events?limit=5"
```

(Exact query API depends on your Relay version — adjust path if needed.)

---

## Auth and TLS

| Variable | Used on | Purpose |
|----------|---------|---------|
| `RELAY_AUTH_TOKEN` | Direct path | `Authorization: Bearer` to Relay `/v1/events` |
| `GATEWAY_AUTH_TOKEN` | Gateway path | Bearer to relay-pubsub (when auth enabled) |
| `RELAY_TLS_INSECURE=1` | Both outbound | Trust self-signed Relay/gateway certs (lab) |
| `EDGE_TLS=1` | Inbound to edge | Serve `/ui` and API over HTTPS |

JWT must be issued by the same Relay instance (or trusted issuer) that receives events. In multi-service deploys, sync the same token to edge and relay-pubsub.

---

## Event families Relay receives

All types are plain strings — Relay policies match on them:

| Family | Example types |
|--------|---------------|
| Farm | `irrigation.required`, `crop.advisory`, `frost.alert`, … |
| Firewater / edge | `firewater.tank.low`, `edge.comms.down`, `telemetry.sample`, … |
| Remote edge | `remote-edge.link.offline`, `remote-edge.galleon.thermal`, … |
| Fleet | `fleet.power.island`, `fleet.robot.lost`, … |

Full cross-family gate (gateway path): [Event matrix](EVENT_MATRIX.md).

Direct mode uses the same types — only the hop between edge and Relay is shorter.

---

## Choosing a path

| Need | Recommendation |
|------|----------------|
| Google Pub/Sub SDKs, pull/subscribe, action gateway | **relay-pubsub** (`GATEWAY_BASE_URL` set) |
| Minimal stack, edge appliance, debugging stamp payload | **Direct** (`GATEWAY_BASE_URL` empty) |
| Kubernetes lab stack | Both pods via `deploy-k8s-remote.sh` (gateway mode) |
| CI / unit tests | Edge only, `publish: false` — no Relay required |

---

## Forge integration (optional)

When **[Forge](https://github.com/zyvorai/forge)** is co-located at an edge site, Relay can use Forge **Decision Records** for human-gated approvals before Act (`decision_backend: forge` on policy). Forge recommends and records; Relay still executes via the Action Gateway. **relay-edge does not call Forge** — it only publishes stamped events.

Typical lab wiring (configure on **Relay**):

```bash
RELAY_FORGE_BASE_URL=http://<forge-host>:30631
RELAY_FORGE_API_KEY=<forge-api-gateway-secret>
```

**Authoritative guide in this repo** → [Integration guide](INTEGRATION.md)

Quick index → [Forge at the edge](FORGE.md)

---

## Related

- [relay](https://github.com/zyvorai/relay) — control plane, `/v1/events`, policies
- [relay-pubsub](https://github.com/zyvorai/relay-pubsub) — Pub/Sub gateway, `relay-events` backend
- [Forge at the edge](FORGE.md) — Forge + relay-edge + decision-making
- [Concepts](CONCEPTS.md) — division of labor, simulators
- [Deployment](DEPLOYMENT.md) — env vars, systemd, k8s
