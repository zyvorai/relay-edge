# relay-edge

**Farm domain, industrial simulators, and stamped events for [Zyvor Relay](https://github.com/zyvorai/relay).**

Relay owns the durable loop — **Accept → Notify → Ack → Act → Verify**.  
relay-edge owns everything upstream: seasons, sites, zones, devices, contacts, telemetry probes, and three rich simulators that publish through [relay-pubsub](https://github.com/zyvorai/relay-pubsub).

```text
   relay-edge              relay-pubsub              Relay
   :18086                  :8081 HTTPS               :8443
   ─────────               ────────────              ─────
   farm · firewater   →    topic = event type   →    policies
   atlas · fleet           relay-events               notify · act
   /ui control rooms       self-signed TLS
```

[![Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)

---

## Why relay-edge?

Relay policies are generic — they match on **event type** and **severity**. Someone has to attach **farm context** before publish: which season, which plot, which valve, who to notify, how to verify the act landed.

relay-edge is that someone. It also ships production-quality **simulators** so you can demo and test the full stack without real hardware.

---

## Quick start

```bash
go test ./...
go run ./cmd/relay-edge
```

| What | Where |
|------|-------|
| Fire-water plant UI | http://127.0.0.1:18086/ui |
| Atlas remote edge | http://127.0.0.1:18086/ui/atlas.html |
| All edge classes | http://127.0.0.1:18086/ui/fleet.html |
| Farm API smoke | `./scripts/smoke.sh` |

**New here?** → [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)

---

## Documentation

| Guide | What's inside |
|-------|---------------|
| [📖 Docs hub](docs/README.md) | Index of everything |
| [🚀 Getting started](docs/GETTING_STARTED.md) | Clone → run → smoke in 5 min |
| [💡 Concepts](docs/CONCEPTS.md) | Stamping, publish paths, stack mental model |
| [🏭 Simulators](docs/SIMULATORS.md) | Firewater, Atlas, Fleet — scenarios & events |
| [📡 Event matrix](docs/EVENT_MATRIX.md) | End-to-end gate for all 4 event families |
| [🚢 Deployment](docs/DEPLOYMENT.md) | systemd, Kubernetes, lab wiring |
| [📋 API reference](docs/API.md) | Every route, one page |

---

## Event families at a glance

| Family | Count | Examples |
|--------|-------|----------|
| Farm | 10 | `irrigation.required`, `crop.advisory`, … |
| Firewater / edge | 18+ | `firewater.tank.low`, `edge.comms.down`, … |
| Atlas | 6 | `atlas.link.offline`, `atlas.galleon.thermal`, … |
| Fleet | 6 | `fleet.power.island`, `fleet.robot.lost`, … |

Verify all four through Relay:

```bash
BASE=https://<relay-host>:8443 \
GATEWAY=https://<gateway-host>:8081 \
EDGE=http://<edge-host>:18086 \
  ./scripts/e2e-events-matrix.sh
```

---

## Deploy anywhere

| Target | One command |
|--------|-------------|
| **Local dev** | `go run ./cmd/relay-edge` |
| **Linux host** | `./scripts/deploy-remote.sh <HOST> [USER]` |
| **Kubernetes** | `./deploy/scripts/deploy-k8s-remote.sh <HOST> [USER]` |

Kubernetes deploys **relay-edge + relay-pubsub** together — both pods with built-in self-signed HTTPS. Details in [Deployment](docs/DEPLOYMENT.md).

---

## Configuration (essentials)

| Variable | Default | Purpose |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen address |
| `EDGE_TLS` | `0` | `1` = self-signed HTTPS |
| `GATEWAY_BASE_URL` | `https://127.0.0.1:8081` | relay-pubsub (preferred) |
| `RELAY_BASE_URL` | `https://127.0.0.1:8443` | Direct Relay fallback |
| `RELAY_AUTH_TOKEN` | — | JWT for gateway + Relay |
| `RELAY_TLS_INSECURE` | `1` | Trust self-signed peers |

Full list → [Deployment](docs/DEPLOYMENT.md#configuration-essentials)

---

## Part of the Zyvor stack

| Project | Role |
|---------|------|
| **[relay](https://github.com/zyvorai/relay)** | Control plane |
| **[relay-pubsub](https://github.com/zyvorai/relay-pubsub)** | Google Pub/Sub → Relay events |
| **relay-edge** (this repo) | Domain + simulators + stamp |

---

## License

Apache-2.0 · Copyright 2026 Zyvor AI Labs
