# relay-edge

**The upstream brain for [Zyvor Relay](https://github.com/zyvorai/relay).**  
Farm topology, industrial simulators, and stamped events — with three browser control rooms you can drive in minutes.

Relay runs the durable loop: **Accept → Notify → Ack → Act → Verify**.  
relay-edge runs everything **before** Accept: seasons, sites, zones, devices, contacts, telemetry probes, and simulators that publish farm-aware events into Relay — via [relay-pubsub](https://github.com/zyvorai/relay-pubsub) or direct.

At sites that also run **[Forge](https://github.com/zyvorai/forge)**, Relay can optionally gate critical acts behind Forge **Decision Records** (human freeze/attest). relay-edge only publishes events — it never calls Forge. → [docs/FORGE.md](docs/FORGE.md)

[![Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev/)

---

## Where this fits

| Layer | Repo | Role at the edge |
|-------|------|------------------|
| **relay-edge** | this repo | Stamp domain context · simulators · `/ui` control rooms |
| **relay-pubsub** | [relay-pubsub](https://github.com/zyvorai/relay-pubsub) | Google Pub/Sub wire → Relay (optional but preferred) |
| **Relay** | [relay](https://github.com/zyvorai/relay) | Notify · Ack · Act · Verify · policies |
| **Forge** | [forge](https://github.com/zyvorai/forge) | GPU/AI/K8s at edge sites · optional Decision Records |

**Two “edges”:** Forge edge = where AI workloads run. relay-edge = where operational events get stamped and published. Same physical site, different jobs.

```text
 ┌──────────────────────────────────────────────────────────────────────┐
 │  Forge edge site (optional)                                           │
 │  Forge :30631 — GPUs · federation · Zeus · Decision Records           │
 │  relay-edge :18086 — farm · firewater · remote-edge · fleet · /ui      │
 └───────────────────────────────┬──────────────────────────────────────┘
                                 │ stamp + publish
              ┌──────────────────┴──────────────────┐
              ▼                                     ▼
   GATEWAY_BASE_URL set                  GATEWAY_BASE_URL empty
   relay-pubsub :8081                    POST /v1/events (direct)
              │                                     │
              └──────────────────┬──────────────────┘
                                 ▼
                          Zyvor Relay :8443
                          Accept → Notify → Ack → Act → Verify
                          optional: Forge approval before Act
```

---

## What you get

| Capability | Details |
|------------|---------|
| **Farm domain API** | Sites, zones, devices, contacts, seasons — JSON on disk, REST CRUD |
| **Stamping** | Every event gets season/site/zone/recipients/verification probe before Relay |
| **Firewater simulator** | 47-point NFPA-style plant + edge AI/comms — `/ui` |
| **Remote-edge simulator** | Distributed site NOC: satellite, compute rack, UAV, vision — `/ui/remote-edge.html` |
| **Fleet simulator** | 60+ devices, 15 edge classes — `/ui/fleet.html` |
| **Two publish paths** | relay-pubsub (production) or direct Relay — [docs/RELAY.md](docs/RELAY.md) |
| **Deploy anywhere** | `go run`, systemd, Kubernetes (pairs with relay-pubsub) |

All simulators: **Seed → Publish into Relay → Scenarios → Live SSE stream**.

---

## Quick start (60 seconds)

```bash
git clone https://github.com/zyvorai/relay-edge.git
cd relay-edge
go test ./...
go run ./cmd/relay-edge
```

| Browser | Control room |
|---------|--------------|
| [http://127.0.0.1:18086/ui](http://127.0.0.1:18086/ui) | Fire-water plant |
| [http://127.0.0.1:18086/ui/remote-edge.html](http://127.0.0.1:18086/ui/remote-edge.html) | Remote edge NOC |
| [http://127.0.0.1:18086/ui/fleet.html](http://127.0.0.1:18086/ui/fleet.html) | All edge classes |

```bash
./scripts/smoke.sh              # farm lifecycle (no Relay required)
./scripts/smoke-firewater.sh    # industrial plant
./scripts/smoke-remote-edge.sh  # remote-edge scenarios
```

**First time?** → [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)

---

## Publish into Relay

### Via relay-pubsub (production)

```bash
export GATEWAY_BASE_URL=https://127.0.0.1:8081
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
```

In any UI: **Seed plant inventory** → **Publish into Relay** → scenario.

### Direct to Relay (minimal stack)

```bash
export GATEWAY_BASE_URL=
export RELAY_BASE_URL=https://127.0.0.1:8443
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
```

Responses show `"path": "relay"` when events hit Relay natively. → [docs/RELAY.md](docs/RELAY.md)

---

## Event families

| Family | Count | Example types |
|--------|-------|---------------|
| **Farm** | 10 | `irrigation.required`, `crop.advisory`, `frost.alert` |
| **Firewater / edge** | 18+ | `firewater.tank.low`, `edge.comms.down` |
| **Remote edge** | 6 | `remote-edge.link.offline`, `remote-edge.galleon.thermal` |
| **Fleet** | 6 | `fleet.power.island`, `fleet.robot.lost` |

Integration gate (all four families → relay-pubsub → Relay):

```bash
BASE=https://<relay>:8443 GATEWAY=https://<gateway>:8081 EDGE=http://<edge>:18086 \
  ./scripts/e2e-events-matrix.sh
```

---

## Forge + decision-making

relay-edge **publishes** stamped events. **Relay** runs the loop. **Forge** (optional) holds the human approval record when policy requires it.

→ **[Integration guide](docs/INTEGRATION.md)** — full architecture, sequence flows, config, walkthroughs

```text
relay-edge event
  → Relay Accept → Notify → Ack
  → [native] operator approve in Relay → Act → Verify
  → [forge]  Relay opens Decision Record → human freeze/attest in Zeus → Act → Verify
```

Configure on **Relay** (not relay-edge):

```bash
RELAY_FORGE_BASE_URL=http://<forge-host>:30631
RELAY_FORGE_API_KEY=<forge-api-gateway-secret>
```

Quick index → [docs/FORGE.md](docs/FORGE.md)

---

## Documentation

| Guide | What's inside |
|-------|---------------|
| [📖 Docs hub](docs/README.md) | Route to the right guide |
| [🚀 Getting started](docs/GETTING_STARTED.md) | Clone → run → smoke in 5 min |
| [🔗 Working with Relay](docs/RELAY.md) | Direct vs gateway, wire contract, Act lifecycle |
| [🤝 Integration guide](docs/INTEGRATION.md) | **relay-edge + Forge + Relay** — flows, config, demos |
| [⚙️ Forge quick index](docs/FORGE.md) | Short Forge pointer |
| [💡 Concepts](docs/CONCEPTS.md) | Stamping, publish paths, division of labor |
| [🏭 Simulators](docs/SIMULATORS.md) | Scenarios, event types, UI workflow |
| [📡 Event matrix](docs/EVENT_MATRIX.md) | Cross-family integration test gate |
| [🚢 Deployment](docs/DEPLOYMENT.md) | systemd, Kubernetes, TLS, lab ports |
| [📋 API reference](docs/API.md) | Every HTTP route |

---

## Deploy

| Target | Command |
|--------|---------|
| **Local** | `go run ./cmd/relay-edge` |
| **Linux host** | `./scripts/deploy-remote.sh <HOST> [USER]` |
| **Kubernetes** | `./deploy/scripts/deploy-k8s-remote.sh <HOST> [USER]` |

k8s deploys **relay-edge + relay-pubsub** together (self-signed HTTPS). → [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)

**Lab co-deploy** (example host `212.8.248.187`): Forge UI `:30862`, Forge gateway `:30631`, Relay `:8443`, pubsub `:8081`, relay-edge `:18086`.

---

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen address |
| `EDGE_TLS` | `0` | `1` = self-signed HTTPS for API + UIs |
| `GATEWAY_BASE_URL` | `https://127.0.0.1:8081` | relay-pubsub (`""` = direct Relay) |
| `RELAY_BASE_URL` | `https://127.0.0.1:8443` | Relay `/v1/events` |
| `RELAY_AUTH_TOKEN` | — | JWT (sync with pubsub + Relay) |
| `RELAY_TLS_INSECURE` | `1` | Trust self-signed TLS (lab) |

---

## Part of the Zyvor stack

| Project | Role |
|---------|------|
| **[relay](https://github.com/zyvorai/relay)** | Control plane — Accept → Notify → Ack → Act → Verify |
| **[relay-pubsub](https://github.com/zyvorai/relay-pubsub)** | Google Pub/Sub compatibility at the edge |
| **relay-edge** (here) | Domain, simulators, stamped publishes |
| **[forge](https://github.com/zyvorai/forge)** | AI/K8s at edge; Decision Records via Relay |

---

## License

Apache-2.0 · Copyright 2026 Zyvor AI Labs
