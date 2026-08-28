# relay-edge documentation

**Stamp farm context. Simulate industrial edge fleets. Publish into Relay.**

---

## Start here

| I want to… | Go to |
|------------|-------|
| Run locally in 2 minutes | [Getting started](GETTING_STARTED.md) |
| Understand the architecture | [Concepts](CONCEPTS.md) |
| **relay-edge + Forge + Relay together** | **[Integration guide](INTEGRATION.md)** |
| Publish into Relay (direct or via pubsub) | [Working with Relay](RELAY.md) |
| Forge at edge sites (quick index) | [Forge](FORGE.md) |
| Deploy to a host or Kubernetes | [Deployment](DEPLOYMENT.md) |
| Drive events through relay-pubsub → Relay | [Event matrix](EVENT_MATRIX.md) |
| Explore firewater / remote-edge / fleet simulators | [Simulators](SIMULATORS.md) |
| Look up HTTP routes | [API reference](API.md) |
| SPDX headers on source | [License headers](LICENSE_HEADERS.md) |

---

## The stack

```text
  relay-edge          relay-pubsub           Zyvor Relay              Forge (optional)
  ───────────         ──────────────         ───────────              ────────────────
  farm domain    →    Pub/Sub REST     →     Accept                   Decision Records
  simulators          topic = type           Notify → Ack → Act        (human gate)
  /ui control rooms   self-signed TLS        Verify
```

relay-edge never replaces Relay — it **feeds** Relay with stamped, policy-ready events. When Forge is co-located, Relay may require Forge freeze/attest before Act — relay-edge does not call Forge; see [FORGE.md](FORGE.md).

---

## What's inside this repo

| Piece | Description |
|-------|-------------|
| **Farm domain** | Sites, zones, devices, contacts, seasons — JSON on disk |
| **Stamping** | Every publish enriched with season/site/zone/recipients/probe |
| **Firewater** | 47-point industrial fire-water plant + edge AI/comms |
| **Remote edge** | Remote-edge NOC: Starlink, Galleon, UAV, vision, IoT |
| **Fleet** | 60+ devices across all edge classes in one catalog |
| **Web UIs** | `/ui`, `/ui/remote-edge.html`, `/ui/fleet.html` |

---

## Scripts cheat sheet

```bash
./scripts/smoke.sh                 # farm lifecycle
./scripts/smoke-firewater.sh       # industrial plant
./scripts/smoke-remote-edge.sh           # remote-edge scenarios
./scripts/e2e-events-matrix.sh     # all 4 families → Relay (needs gateway + Relay)
./scripts/deploy-remote.sh HOST    # systemd deploy
./deploy/scripts/deploy-k8s-remote.sh HOST   # k8s stack (+ sibling relay-pubsub)
```

---

## Related projects

- [relay-pubsub](https://github.com/zyvorai/relay-pubsub) — Google Pub/Sub gateway → Relay
- [relay](https://github.com/zyvorai/relay) — control plane
- [forge](https://github.com/zyvorai/forge) — AI/K8s at edge sites; Decision Records with Relay
