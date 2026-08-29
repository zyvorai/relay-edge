# relay-edge documentation

**Stamp site context for all IoT families. Simulate farm, firewater, remote-edge & fleet. Publish into Relay.**

---

## Start here

| I want to… | Go to |
|------------|-------|
| Run locally in 2 minutes | [Getting started](GETTING_STARTED.md) |
| Understand the architecture | [Concepts](CONCEPTS.md) |
| **relay-edge + Forge + Relay together** | **[Integration guide](INTEGRATION.md)** · **[Stack without Forge (diagrams)](INTEGRATION.md#stack-without-forge-default)** |
| Simulate full stack (Forge optional) | `./scripts/e2e-stack.sh` / `./scripts/e2e-direct-stack.sh` — see [Integration](INTEGRATION.md#simulate-all-one-command) |
| **Lab test results (what we ran)** | **[Test results](TEST_RESULTS.md)** · [/ui/docs.html](/ui/docs.html) |
| Publish into Relay (direct or via pubsub) | [Working with Relay](RELAY.md) |
| Deploy to a host or Kubernetes | [Deployment](DEPLOYMENT.md) |
| Cut a release / pull GHCR image | [Deployment § CI and releases](DEPLOYMENT.md#ci-and-releases) · [Releases](https://github.com/zyvorai/relay-edge/releases) |
| Drive events through relay-pubsub → Relay | [Event matrix](EVENT_MATRIX.md) |
| Explore firewater / remote-edge / fleet simulators | [Simulators](SIMULATORS.md) |
| Look up HTTP routes | [API reference](API.md) |
| Environment variables | [Configuration](CONFIGURATION.md) |
| Lab Act wiring (TLS / targets) | [`lab-wire-relay-act.sh`](../scripts/lab-wire-relay-act.sh) · [TEST_RESULTS](TEST_RESULTS.md) |
| Contribute / report security | [Contributing](../CONTRIBUTING.md) · [Security](../SECURITY.md) |
| SPDX headers on source | [License headers](LICENSE_HEADERS.md) |

---

## The stack

```text
  relay-edge          relay-pubsub           Zyvor Relay              Forge (optional)
  ───────────         ──────────────         ───────────              ────────────────
  domain + sims  →    Pub/Sub REST     →     Accept                   Decision Records
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
| **Remote edge** | 24 assets: Starlink, Galleon, UAV, vision, yard IoT |
| **Fleet** | 77 devices across **18** edge classes in one catalog |
| **Web UIs** | `/ui`, `/ui/remote-edge.html`, `/ui/fleet.html`, `/ui/docs.html` |
| **CI / release** | GitHub Actions: vet + unit + 4 smokes; tag-gated binaries + `ghcr.io/zyvorai/relay-edge` |

---

## Scripts cheat sheet

```bash
./scripts/smoke.sh                 # farm lifecycle
./scripts/smoke-firewater.sh       # industrial plant
./scripts/smoke-remote-edge.sh     # remote-edge scenarios
./scripts/smoke-fleet.sh           # fleet catalog + scenarios
make smoke-all                     # all four smokes (EDGE=…)
./scripts/e2e-events-matrix.sh     # all 4 families → Relay
./scripts/e2e-direct-relay.sh      # direct Relay (no pubsub, expanded scenarios)
./scripts/e2e-direct-stack.sh      # direct: probe + scenario matrix
./scripts/stack-probe.sh           # health: edge + pubsub + Relay (+ Forge)
./scripts/stack-probe.sh --direct  # health: edge + Relay only
./scripts/e2e-stack.sh             # no Forge: probe + event matrix
./scripts/e2e-forge-stack.sh       # matrix + Forge path when FORGE_* set
./scripts/lab-wire-relay-act.sh HOST  # wire Relay Act → pubsub (TLS insecure)
./scripts/deploy-remote.sh HOST    # systemd (or nohup fallback)
./deploy/scripts/deploy-k8s-remote.sh HOST   # k8s stack (+ sibling relay-pubsub)
```

**CI:** PR/push → vet, `go test`, local smokes (incl. fleet). **Release:** push `v*` or Actions → Release (binaries + GHCR).

**Latest lab verification:** [TEST_RESULTS.md](TEST_RESULTS.md) (2026-08-29 — **all PASS**, gateway + direct). Browser summary: `/ui/docs.html`.

---

## Related projects

- [relay-pubsub](https://github.com/zyvorai/relay-pubsub) — Google Pub/Sub gateway → Relay
- [relay](https://github.com/zyvorai/relay) — control plane
- [forge](https://github.com/zyvorai/forge) — AI/K8s at edge sites; Decision Records with Relay
- [Contributing](../CONTRIBUTING.md) · [Security](../SECURITY.md) · [Releases](https://github.com/zyvorai/relay-edge/releases)