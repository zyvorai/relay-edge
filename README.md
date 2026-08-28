# relay-edge

**The upstream brain for [Zyvor Relay](https://github.com/zyvorai/relay).**  
Farm topology, industrial simulators, and stamped events — with three browser control rooms you can drive in minutes.

Relay runs the durable loop: **Accept → Notify → Ack → Act → Verify**.  
relay-edge runs everything before that: seasons, sites, zones, devices, contacts, telemetry probes, and rich simulators that publish farm-aware events into Relay — directly or through [relay-pubsub](https://github.com/zyvorai/relay-pubsub).

At sites that also run **[Forge](https://github.com/zyvorai/forge)**, Relay can optionally route critical approvals through Forge **Decision Records** before Act — see [docs/FORGE.md](docs/FORGE.md).

[![Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev/)

```text
 ┌─────────────────────────────────────────────────────────────────────┐
 │                        relay-edge  :18086                             │
 │  farm domain · firewater · remote-edge · fleet · /ui control rooms        │
 └───────────────────────────────┬─────────────────────────────────────┘
                                 │ stamp + publish
              ┌──────────────────┴──────────────────┐
              │                                     │
              ▼                                     ▼
   GATEWAY_BASE_URL set                  GATEWAY_BASE_URL empty
              │                                     │
              ▼                                     ▼
   relay-pubsub  :8081                   POST /v1/events
   topic = event type                    (Relay direct)
              │                                     │
              └──────────────────┬──────────────────┘
                                 ▼
                          Zyvor Relay  :8443
                          policies · notify · act · verify
                          optional: Forge Decision Records
```

---

## What you get

| Capability | Details |
|------------|---------|
| **Farm domain API** | Sites, zones, devices, contacts, seasons — JSON on disk, REST CRUD |
| **Stamping** | Every event enriched with season/site/zone/recipients/verification probe before Relay sees it |
| **Firewater simulator** | 47-point NFPA-style plant + edge AI/comms — full control room at `/ui` |
| **Remote-edge simulator** | Remote-edge NOC: Starlink, Galleon, UAV, vision — `/ui/remote-edge.html` |
| **Fleet simulator** | 60+ devices, 15 edge classes — `/ui/fleet.html` |
| **Two publish paths** | Via relay-pubsub (preferred) or direct to Relay — [docs/RELAY.md](docs/RELAY.md) |
| **Deploy anywhere** | `go run`, systemd, Kubernetes (pairs with relay-pubsub) |

All three simulators share the same UX pattern: **Seed → Publish toggle → Scenarios → Live event stream**.

---

## Quick start (60 seconds)

```bash
git clone https://github.com/zyvorai/relay-edge.git
cd relay-edge
go test ./...
go run ./cmd/relay-edge
```

| Open in browser | What it is |
|-----------------|------------|
| [http://127.0.0.1:18086/ui](http://127.0.0.1:18086/ui) | Fire-water control room |
| [http://127.0.0.1:18086/ui/remote-edge.html](http://127.0.0.1:18086/ui/remote-edge.html) | Remote edge NOC (satellite, compute, UAV) |
| [http://127.0.0.1:18086/ui/fleet.html](http://127.0.0.1:18086/ui/fleet.html) | All edge classes |

Farm API smoke (no Relay required):

```bash
./scripts/smoke.sh
```

**First time?** → [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)

---

## Publish into Relay

### Via relay-pubsub (production path)

```bash
export GATEWAY_BASE_URL=https://127.0.0.1:8081
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
```

In any UI: **Seed plant inventory** → enable **Publish into Relay** → pick a scenario.

### Direct to Relay (minimal stack)

```bash
export GATEWAY_BASE_URL=
export RELAY_BASE_URL=https://127.0.0.1:8443
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
```

Responses show `"path": "relay"` when events hit Relay natively. Full wire format and lifecycle → **[docs/RELAY.md](docs/RELAY.md)**.

---

## Event families

| Family | Types | Origin |
|--------|-------|--------|
| **Farm** | 10 | Season lifecycle, advisories, critical irrigation |
| **Firewater / edge** | 18+ | Industrial plant + edge AI/comms simulator |
| **Remote edge** | 6 | Starlink, Galleon, UAV, vision, IoT |
| **Fleet** | 6 | AMR, energy, OT, building, marine, security, … |

End-to-end integration gate (all four families through relay-pubsub → Relay):

```bash
BASE=https://<relay-host>:8443 \
GATEWAY=https://<gateway-host>:8081 \
EDGE=http://<edge-host>:18086 \
  ./scripts/e2e-events-matrix.sh
```

---

## Documentation

| Guide | What's inside |
|-------|---------------|
| [📖 Docs hub](docs/README.md) | Start here — routes you to the right guide |
| [🚀 Getting started](docs/GETTING_STARTED.md) | Clone → run → smoke in 5 min |
| [🔗 Working with Relay](docs/RELAY.md) | Direct vs gateway, wire contract, Act lifecycle |
| [⚙️ Forge at the edge](docs/FORGE.md) | Forge + relay-edge + Relay, decision-making |
| [💡 Concepts](docs/CONCEPTS.md) | Stamping, publish paths, mental model |
| [🏭 Simulators](docs/SIMULATORS.md) | Scenarios, events, UI workflow |
| [📡 Event matrix](docs/EVENT_MATRIX.md) | Cross-family integration test gate |
| [🚢 Deployment](docs/DEPLOYMENT.md) | systemd, Kubernetes, TLS, env vars |
| [📋 API reference](docs/API.md) | Every HTTP route |

---

## Deploy

| Target | Command |
|--------|---------|
| **Local dev** | `go run ./cmd/relay-edge` |
| **Linux host** | `./scripts/deploy-remote.sh <HOST> [USER]` |
| **Kubernetes** | `./deploy/scripts/deploy-k8s-remote.sh <HOST> [USER]` |

The k8s script deploys **relay-edge + relay-pubsub** together — both pods with built-in self-signed HTTPS. See [Deployment](docs/DEPLOYMENT.md).

---

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen address |
| `EDGE_TLS` | `0` | `1` = self-signed HTTPS for edge API + UIs |
| `GATEWAY_BASE_URL` | `https://127.0.0.1:8081` | relay-pubsub (empty = direct Relay) |
| `RELAY_BASE_URL` | `https://127.0.0.1:8443` | Relay direct `/v1/events` |
| `RELAY_AUTH_TOKEN` | — | JWT for Relay and/or gateway |
| `RELAY_TLS_INSECURE` | `1` | Trust self-signed TLS peers (lab) |

---

## Part of the Zyvor stack

```text
  relay          relay-pubsub       relay-edge (this repo)     forge (optional)
  ─────          ──────────────     ──────────────────────     ────────────────
  control        Pub/Sub gateway    domain + simulators        AI/K8s at edge
  plane          → Relay events     + stamp + UIs              Decision Records
```

| Project | Role |
|---------|------|
| **[relay](https://github.com/zyvorai/relay)** | Control plane — Accept → Notify → Ack → Act → Verify |
| **[relay-pubsub](https://github.com/zyvorai/relay-pubsub)** | Google Pub/Sub compatibility layer |
| **relay-edge** (you are here) | Farm domain, simulators, stamped publishes |
| **[forge](https://github.com/zyvorai/forge)** | GPU/AI infra at edge sites; optional human-gated approvals via Relay |

---

## License

Apache-2.0 · Copyright 2026 Zyvor AI Labs
