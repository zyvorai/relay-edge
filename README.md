# relay-edge

**The upstream brain for [Zyvor Relay](https://github.com/zyvorai/relay).**  
Site topology, four IoT simulators, and stamped events — with three browser control rooms you can drive in minutes.

Relay runs the durable loop: **Accept → Notify → Ack → Act → Verify**.  
relay-edge runs everything **before** Accept: seasons, sites, zones, devices, contacts, telemetry probes, and simulators that publish stamped events into Relay — via [relay-pubsub](https://github.com/zyvorai/relay-pubsub) or direct.

At sites that also run **[Forge](https://github.com/zyvorai/forge)**, Relay can optionally gate critical acts behind Forge **Decision Records** (human freeze/attest). relay-edge only publishes events — it never calls Forge. → [docs/FORGE.md](docs/FORGE.md)

[![CI](https://github.com/zyvorai/relay-edge/actions/workflows/ci.yml/badge.svg)](https://github.com/zyvorai/relay-edge/actions/workflows/ci.yml)
[![Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev/)

**CI:** every PR/push runs vet, unit tests, and local smoke (farm, firewater, remote-edge, fleet). Releases are **tag-gated** (`v*`): GitHub Release binaries (linux/darwin × amd64/arm64) + `ghcr.io/zyvorai/relay-edge`. Cut one via Actions → **Release** → Run workflow, or `git tag vX.Y.Z && git push --tags`.

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
| **Fleet simulator** | 77 devices, 18 edge classes — `/ui/fleet.html` |
| **Two publish paths** | relay-pubsub (production) or direct Relay — [docs/RELAY.md](docs/RELAY.md) |
| **Deploy anywhere** | `go run`, systemd, Kubernetes (pairs with relay-pubsub) |

All simulators: **Seed → Publish into Relay → Scenarios → Live SSE stream**.

---

## Quick start (60 seconds)

```bash
go test ./...
go run ./cmd/relay-edge   # HTTPS by default (self-signed under ./data/tls)
# open https://127.0.0.1:18086/ui/  (accept certificate warning)
# EDGE_TLS=0 for plain HTTP
```

| Browser | Control room |
|---------|--------------|
| [https://127.0.0.1:18086/ui](https://127.0.0.1:18086/ui) | Home · configure · lab · logs |
| [https://127.0.0.1:18086/ui/firewater.html](https://127.0.0.1:18086/ui/firewater.html) | Fire-water plant |
| [https://127.0.0.1:18086/ui/remote-edge.html](https://127.0.0.1:18086/ui/remote-edge.html) | Remote edge NOC |
| [https://127.0.0.1:18086/ui/fleet.html](https://127.0.0.1:18086/ui/fleet.html) | All edge classes |
| [https://127.0.0.1:18086/ui/docs.html](https://127.0.0.1:18086/ui/docs.html) | Docs & stack test results |

```bash
./scripts/smoke.sh              # farm lifecycle (no Relay required)
./scripts/smoke-firewater.sh    # industrial plant
./scripts/smoke-remote-edge.sh  # remote-edge scenarios
./scripts/smoke-fleet.sh        # fleet catalog + blackout / amr_lost
make smoke-all                  # all four (EDGE=http://127.0.0.1:18086)
./scripts/e2e-direct-relay.sh   # direct Relay — expanded scenarios (no pubsub)
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
| **Firewater / edge** | 20+ | `firewater.tank.low`, `edge.comms.down`, `edge.vision.fire` |
| **Remote edge** | 6 | `remote-edge.link.offline`, `remote-edge.galleon.thermal` |
| **Fleet** | 6 | `fleet.power.island`, `fleet.robot.lost` |

Integration gate (all four families → relay-pubsub → Relay):

```bash
BASE=https://<relay>:8443 GATEWAY=https://<gateway>:8081 EDGE=http://<edge>:18086 \
  ./scripts/e2e-events-matrix.sh
```

**Direct to Relay** (no relay-pubsub — edge `POST /v1/events`):

```bash
BASE=https://<relay>:8443 EDGE=http://<edge>:18086 RELAY_AUTH_TOKEN=<jwt> \
  ./scripts/e2e-direct-relay.sh
```

See [config/lab-direct.env.example](config/lab-direct.env.example) and [docs/RELAY.md](docs/RELAY.md#try-direct-mode-locally). One-liner: `./scripts/e2e-direct-stack.sh`.

---

## Simulate full stack

When Relay, relay-pubsub, and relay-edge are running (**Forge optional**):

```bash
cp config/lab-stack.env.example config/lab-stack.env
# fill RELAY_AUTH_TOKEN; FORGE_* only if Forge is deployed

set -a && source config/lab-stack.env && set +a
./scripts/e2e-stack.sh              # no Forge required
# ./scripts/e2e-forge-stack.sh      # when Forge is co-located
```

→ [Integration guide](docs/INTEGRATION.md#simulate-all-one-command)

**Verified on lab** (2026-08-29): gateway + direct **PASS** (farm Act `rpg_*` included) — see [docs/TEST_RESULTS.md](docs/TEST_RESULTS.md) or `/ui/docs.html`.

---

## Forge + decision-making

relay-edge **publishes** stamped events. **Relay** runs the loop. **Forge** (external sibling repo) holds optional human approval records.

→ **[Integration guide](docs/INTEGRATION.md)** — architecture, glossary, simulation scripts

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

---

## Documentation

| Guide | What's inside |
|-------|---------------|
| [📖 Docs hub](docs/README.md) | Route to the right guide |
| [🚀 Getting started](docs/GETTING_STARTED.md) | Clone → run → smoke in 5 min |
| [✅ Test results](docs/TEST_RESULTS.md) | **Lab verification** — what we tested, how, outcomes |
| [📋 API reference](docs/API.md) | Every HTTP route + stamping pipeline |
| [⚙️ Configuration](docs/CONFIGURATION.md) | All environment variables and publish paths |
| [🔗 Working with Relay](docs/RELAY.md) | Direct vs gateway, wire contract, Act lifecycle |
| [🤝 Integration guide](docs/INTEGRATION.md) | **relay-edge + Relay + Forge** (sibling) — simulate all |
| [💡 Concepts](docs/CONCEPTS.md) | Stamping, publish paths, division of labor |
| [🏭 Simulators](docs/SIMULATORS.md) | Scenarios, event types, UI workflow |
| [📡 Event matrix](docs/EVENT_MATRIX.md) | Cross-family integration test gate |
| [🚢 Deployment](docs/DEPLOYMENT.md) | systemd, Kubernetes, TLS, lab ports |

---

## Deploy

| Target | Command |
|--------|---------|
| **Local** | `go run ./cmd/relay-edge` |
| **Linux host** | `./scripts/deploy-remote.sh <HOST> [USER]` (systemd or nohup) |
| **Container** | `ghcr.io/zyvorai/relay-edge:latest` (or `:0.1.1`) |
| **Kubernetes** | `./deploy/scripts/deploy-k8s-remote.sh <HOST> [USER]` |

k8s deploys **relay-edge + relay-pubsub** together (self-signed HTTPS). → [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)

**Lab co-deploy** (single `<host>`): Forge UI `:30862`, Forge gateway `:30631`, Relay `:8443`, pubsub `:8081`, relay-edge `:18086`.

---

## Configuration

Full reference → **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)**

| Variable | Default | Purpose |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen address |
| `EDGE_DATA_DIR` | `./data` | JSON stores (seasons, sites, zones, devices, contacts) |
| `EDGE_TLS` | `0` | `1` = self-signed HTTPS for API + UIs |
| `EDGE_TLS_CERT` / `EDGE_TLS_KEY` / `EDGE_TLS_SAN` | see docs | TLS paths and SANs |
| `EDGE_ENABLED_FAMILIES` | _(all)_ | Optional: `firewater`, `remote-edge`, `fleet` |
| `GATEWAY_BASE_URL` | `https://127.0.0.1:8081` if **unset** | relay-pubsub. For **direct Relay**, set explicitly empty: `export GATEWAY_BASE_URL=` (unset ≠ direct) |
| `GATEWAY_AUTH_TOKEN` | — | Optional gateway JWT |
| `RELAY_BASE_URL` | `https://127.0.0.1:18080` | Relay `/v1/events` (direct path). Use `:8443` in lab/production. |
| `RELAY_AUTH_TOKEN` | — | JWT (sync with pubsub + Relay) |
| `RELAY_TLS_INSECURE` | `1` | Trust self-signed TLS (lab) |
| `FASAL_GCP_PROJECT` | `fasal-onprem` | GCP project in gateway publish URL |

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
