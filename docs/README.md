# relay-edge documentation

**Stamp farm context. Simulate industrial edge fleets. Publish into Relay.**

---

## Start here

| I want to… | Go to |
|------------|-------|
| Run locally in 2 minutes | [Getting started](GETTING_STARTED.md) |
| Understand the architecture | [Concepts](CONCEPTS.md) |
| Publish into Relay (direct or via pubsub) | [Working with Relay](RELAY.md) |
| Deploy to a host or Kubernetes | [Deployment](DEPLOYMENT.md) |
| Drive events through relay-pubsub → Relay | [Event matrix](EVENT_MATRIX.md) |
| Explore firewater / atlas / fleet simulators | [Simulators](SIMULATORS.md) |
| Look up HTTP routes | [API reference](API.md) |

---

## The stack

```text
  relay-edge          relay-pubsub           Zyvor Relay
  ───────────         ──────────────         ───────────
  farm domain    →    Pub/Sub REST     →     Accept
  simulators          topic = type           Notify → Ack
  /ui control rooms   self-signed TLS        Act → Verify
```

relay-edge never replaces Relay — it **feeds** Relay with stamped, policy-ready events.

---

## What's inside this repo

| Piece | Description |
|-------|-------------|
| **Farm domain** | Sites, zones, devices, contacts, seasons — JSON on disk |
| **Stamping** | Every publish enriched with season/site/zone/recipients/probe |
| **Firewater** | 47-point industrial fire-water plant + edge AI/comms |
| **Atlas** | Remote-edge NOC: Starlink, Galleon, UAV, vision, IoT |
| **Fleet** | 60+ devices across all edge classes in one catalog |
| **Web UIs** | `/ui`, `/ui/atlas.html`, `/ui/fleet.html` |

---

## Scripts cheat sheet

```bash
./scripts/smoke.sh                 # farm lifecycle
./scripts/smoke-firewater.sh       # industrial plant
./scripts/smoke-atlas.sh           # atlas scenarios
./scripts/e2e-events-matrix.sh     # all 4 families → Relay (needs gateway + Relay)
./scripts/deploy-remote.sh HOST    # systemd deploy
./deploy/scripts/deploy-k8s-remote.sh HOST   # k8s stack (+ sibling relay-pubsub)
```

---

## Related projects

- [relay-pubsub](https://github.com/zyvorai/relay-pubsub) — Google Pub/Sub gateway → Relay
- [relay](https://github.com/zyvorai/relay) — control plane
