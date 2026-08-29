# Stack test results

Live verification of relay-edge + relay-pubsub + Relay (+ Forge) on a co-deployed lab stack.

← [Docs hub](README.md) · [Integration guide](INTEGRATION.md) · [Event matrix](EVENT_MATRIX.md) · [Browser docs](/ui/docs.html)

**Last re-run:** 2026-08-29 · **Outcome:** all gates **PASS**

| Path | Script | Result |
|------|--------|--------|
| Unit tests | `go test ./...` | **PASS** |
| Smoke | `smoke.sh` · `smoke-firewater.sh` · `smoke-remote-edge.sh` | **PASS** |
| Gateway stack | `e2e-stack.sh` | **PASS** — farm 10/10 Accept + **5/5 Act** · FW 5 · remote-edge 6 · fleet 6 |
| Direct Relay | `e2e-direct-stack.sh` | **PASS** — farm 10 · firewater 13 · remote-edge 6 · fleet 6 |

**Act wiring (required for Farm Act):** Relay binary with outbound `RELAY_TLS_INSECURE` ([relay#8bef494](https://github.com/zyvorai/relay/commit/8bef494)) and all four controllers → `https://127.0.0.1:8081/v1/actions`. Helper: [`scripts/lab-wire-relay-act.sh`](../scripts/lab-wire-relay-act.sh).

---

## What we tested

End-to-end proof that **all four event families** from relay-edge reach Relay — via **relay-pubsub** (gateway) or **direct** `POST /v1/events` — plus Act via the pubsub Action Gateway (`rpg_*`) and optional Forge Decision Records.

| Layer | Verified behaviour |
|-------|-------------------|
| **relay-edge** | Farm seasons/sites + firewater plant + remote-edge NOC + fleet IoT simulators; stamp + publish |
| **relay-pubsub** | 40-topic catalog; Pub/Sub REST → Relay; inbound `/v1/actions` for all controller targets |
| **Relay** | Accept, notify, ack, act, verify for every family; Forge `awaiting_decision` only when policy + Forge configured |
| **Forge** | Decision Record create (Relay), human freeze Approved → act; Rejected → failed (optional) |

---

## Lab topology (typical ports)

| Service | URL pattern | Repo |
|---------|-------------|------|
| relay-edge | `http://<host>:18086` | relay-edge |
| relay-pubsub | `https://<host>:8081` | relay-pubsub |
| Relay | `https://<host>:8443` | relay |
| Forge gateway | `http://<host>:30631` | forge |
| Forge UI (Zeus) | `http://<host>:30862` | forge |

Relay process env (on the lab host, loopback to co-located services):

```bash
RELAY_TENANT=fasal-edge
RELAY_ACTION_TARGETS=farm-controller=https://127.0.0.1:8081/v1/actions,\
firewater-controller=https://127.0.0.1:8081/v1/actions,\
remote-edge-controller=https://127.0.0.1:8081/v1/actions,\
fleet-controller=https://127.0.0.1:8081/v1/actions
RELAY_TLS_INSECURE=1          # outbound action HTTPS to self-signed pubsub
RELAY_FORGE_BASE_URL=http://127.0.0.1:30631
RELAY_FORGE_API_KEY=<k8s forge-api-gateway-secret>
```

pubsub and relay-edge share the same `RELAY_AUTH_TOKEN` (JWT from `demo`/`demo` login).

Set external URLs in `config/lab-stack.env` (from `config/lab-stack.env.example`).

---

## How we tested

### 1. Prepare environment

```bash
git clone https://github.com/zyvorai/relay-edge.git && cd relay-edge
cp config/lab-stack.env.example config/lab-stack.env
# Edit BASE, GATEWAY, EDGE, FORGE_BASE for your lab host
```

Fill in `config/lab-stack.env`:

| Variable | Source |
|----------|--------|
| `RELAY_AUTH_TOKEN` | `curl -fsSk -X POST $BASE/v1/auth/login -d '{"username":"demo","password":"demo"}'` → `.token` |
| `FORGE_API_KEY` | `kubectl -n forge get secret forge-api-gateway-secret -o jsonpath='{.data.api-key}' \| base64 -d` |

Ensure Relay has `RELAY_FORGE_*` and `RELAY_TLS_INSECURE=1` (see above). After any Relay restart, re-sync pubsub:

```bash
# on lab host
sudo sed -i "s|^RELAY_AUTH_TOKEN=.*|RELAY_AUTH_TOKEN=$TOKEN|" /etc/relay-pubsub/relay-pubsub.env
sudo systemctl restart relay-pubsub
```

Redeploy relay-edge if needed:

```bash
RELAY_AUTH_TOKEN=$TOKEN ./scripts/deploy-remote.sh <HOST> [USER]
```

### 2. Health probe

```bash
set -a && source config/lab-stack.env && set +a
./scripts/stack-probe.sh
```

Checks: relay-edge `/healthz`, pubsub `/healthz`, Relay `/healthz`, Forge `POST /api/zeus/decisions` probe.

### 3. Event matrix (integration gate)

```bash
./scripts/e2e-events-matrix.sh
```

| Phase | Cases | Pass criteria |
|-------|-------|---------------|
| **A. Farm** | 10 catalog types via gateway REST | 5 critical → `pol_critical_farm` + ack + Act `executed` with `rpg_*` provider; 5 advisory → `pol_advisory`, notify-only |
| **B. Firewater** | 5 scenarios | New Relay event per type (`firewater.*`, `edge.*`) |
| **C. Remote edge** | 6 scenarios (incl. `drone_patrol`) | New Relay event per type (`remote-edge.*`) |
| **D. Fleet** | 6 scenarios | New Relay event per type (`fleet.*`) |

### 4. Full stack + Forge path

```bash
./scripts/e2e-forge-stack.sh
```

Runs the event matrix (Phase A), then:

| Phase | Action |
|-------|--------|
| **B** | Patch `pol_critical_farm` → `decision_backend: forge` (restored on exit) |
| **C** | relay-edge farm publish `irrigation.required` |
| **D** | Operator ack **approve** in Relay → `awaiting_decision` |
| **E–F** | Forge freeze **Approved** → Relay resumes → act/verify |
| **G** | Second event → Forge freeze **Rejected** → Relay `failed` |

Skip flags:

- `./scripts/stack-probe.sh --forge-optional` — when Forge is not deployed
- `./scripts/e2e-forge-stack.sh --skip-matrix` — Forge path only

---

## Without Forge (tested 2026-08-28)

Most edge sites run **relay-edge + relay-pubsub + Relay** only — all four event families (farm, firewater/edge IoT, remote edge, fleet/multi-IoT), not farm alone. Forge is optional for human Decision Records.

### Plan

| Step | What | Families | Requires Forge? |
|------|------|----------|-------------------|
| 1 | Health: edge, pubsub, Relay | all | No |
| 2 | Farm catalog via gateway REST | farm (10 types, 5 Act) | No |
| 3 | Firewater / edge IoT scenarios | industrial plant + edge AI/comms | No |
| 4 | Remote edge scenarios | Starlink, Galleon, UAV, vision, IoT | No |
| 5 | Fleet scenarios | AMR, OT, energy, BMS, security, … | No |
| 6 | Native ack → act (all stamped targets) | farm · firewater · remote-edge · fleet | No |
| 7 | Forge freeze approve/reject | farm (policy patch in e2e) | **Yes** |

Most edge sites run **relay-edge + relay-pubsub + Relay** only. Forge is optional.

→ **[Stack without Forge — architecture, sequence, state diagrams](INTEGRATION.md#stack-without-forge-default)** · [Test results](TEST_RESULTS.md#without-forge-tested-2026-08-28)

Leave `FORGE_BASE` and `FORGE_API_KEY` empty in `config/lab-stack.env`. Do not set `RELAY_FORGE_*` on Relay unless you need step 5.

### How to run

```bash
cp config/lab-stack.env.example config/lab-stack.env
# BASE, GATEWAY, EDGE, RELAY_AUTH_TOKEN only — leave FORGE_* blank

set -a && source config/lab-stack.env && set +a
./scripts/e2e-stack.sh
```

Alternative (same coverage):

```bash
./scripts/stack-probe.sh --forge-optional
./scripts/e2e-events-matrix.sh
# or: ./scripts/e2e-forge-stack.sh   # skips Forge phases B–G automatically
```

### Results (2026-08-28 morning — Accept + Act OK)

| Script | Result |
|--------|--------|
| `stack-probe.sh --forge-optional` | **PASS** — Forge skipped |
| `e2e-events-matrix.sh` | **PASS** — A farm 10/10 Accept + 5/5 Act · B firewater 5 · C remote-edge 5 · D fleet 6 |
| `e2e-stack.sh` | **PASS** — probe + matrix |
| `e2e-forge-stack.sh` | **PASS** — matrix only; Forge path skipped |

### Re-run (2026-08-28 → 2026-08-29 — Act wiring fix)

After Farm Act failed with `tls: certificate signed by unknown authority`, we redeployed Relay (`RELAY_TLS_INSECURE=1` honored) and set action targets. Final verification **2026-08-29**:

| Script | Result |
|--------|--------|
| `stack-probe.sh --forge-optional` | **PASS** |
| `e2e-events-matrix.sh` | **PASS** — A farm **10/10 Accept + 5/5 Act** · B firewater 5 · C remote-edge **6** · D fleet 6 |
| `e2e-stack.sh` | **PASS** |
| `e2e-direct-stack.sh` | **PASS** |

Wire Act on a fresh lab:

```bash
# From a checkout of zyvorai/relay (commit 8bef494+):
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/relay-linux ./cmd/relay
RELAY_BIN=/tmp/relay-linux ./scripts/lab-wire-relay-act.sh <HOST>
# then sync pubsub JWT + gateway deploy + e2e-stack.sh
```

If `FORGE_BASE` is set but Forge is down, `e2e-forge-stack.sh` still **PASS** after the event matrix (Forge phases skipped with warning).

---

## Direct Relay (tested 2026-08-28)

relay-edge → Relay **without relay-pubsub** (`GATEWAY_BASE_URL` empty on edge). Accept-only gate — Act requires pubsub Action Gateway.

### How to run

```bash
cp config/lab-direct.env.example config/lab-direct.env
# BASE, EDGE, RELAY_AUTH_TOKEN

RELAY_EDGE_DIRECT=1 RELAY_AUTH_TOKEN=<jwt> ./scripts/deploy-remote.sh <HOST> [USER]

set -a && source config/lab-direct.env && set +a
./scripts/stack-probe.sh --direct
./scripts/e2e-direct-relay.sh
# or: ./scripts/e2e-direct-stack.sh
```

Restore gateway mode after test:

```bash
RELAY_AUTH_TOKEN=<jwt> ./scripts/deploy-remote.sh <HOST> [USER]
```

### Scenario coverage

| Section | Cases | Pass criteria |
|---------|-------|---------------|
| **0. Guard** | open season | `publish.path == "relay"` |
| **A. Farm** | 10 types via season API | Accept in Relay; path `relay` |
| **B. Firewater** | 13 scenarios | New Relay event per type |
| **C. Remote edge** | 6 scenarios (+ 2 readings-only skip) | New Relay event per type |
| **D. Fleet** | 6 scenarios (+ 2 readings-only skip) | New Relay event per type |

### Results (2026-08-29, lab)

| Script | Result |
|--------|--------|
| `stack-probe.sh --direct` | **PASS** |
| `e2e-direct-relay.sh` | **PASS** — farm 10 · firewater 13 · remote-edge 6 · fleet 6 |
| `e2e-direct-stack.sh` | **PASS** when Relay stays up for the full run |

**Transient:** Relay briefly unreachable mid-matrix once (`curl: Couldn't connect` on `:8443`); re-run after recovery was full **PASS**. Always restore gateway mode after direct tests (`deploy-remote.sh` **without** `RELAY_EDGE_DIRECT`).

---

## With Forge (tested 2026-08-28)

### stack-probe.sh

```
  ok  relay-edge $EDGE/healthz
  ok  relay-pubsub $GATEWAY/healthz
  ok  Relay $BASE/healthz
  ok  Forge $FORGE_BASE/api/zeus/decisions
PASS: stack probe
```

### e2e-events-matrix.sh

| Section | Result |
|---------|--------|
| A. Farm catalog | **10/10** Accept; **5/5** Act via `rpg_*` |
| B. Firewater / edge | **5/5** events in Relay |
| C. Remote edge | **6/6** events in Relay (incl. `drone_patrol`) |
| D. Fleet | **6/6** events in Relay |

### e2e-forge-stack.sh

| Phase | Result |
|-------|--------|
| A. Event matrix | PASS |
| B. Policy → forge backend | PASS (restored on exit) |
| C. relay-edge publish | PASS |
| D. ack → awaiting_decision | PASS |
| E–F. Forge Approved → act | PASS (`verifying` / `action_pending`) |
| G. Forge Rejected → failed | PASS |

**Exit code:** 0

---

## Issues hit during verification

| Symptom | Fix |
|---------|-----|
| Gateway publish `500` / `401` | Re-sync `RELAY_AUTH_TOKEN` on pubsub after Relay restart; restart `relay-pubsub` |
| Farm Act `failed` — TLS unknown authority | Relay binary must honor `RELAY_TLS_INSECURE=1` ([relay#8bef494](https://github.com/zyvorai/relay/commit/8bef494)); run [`lab-wire-relay-act.sh`](../scripts/lab-wire-relay-act.sh) |
| Farm Act `failed` — mock targets only | Set all controllers to `https://127.0.0.1:8081/v1/actions` (not `mock://`) |
| Action circuit breaker open | Reset by fixing TLS; publish fresh catalog events |
| `publish.path=gateway` when expecting direct | Set `GATEWAY_BASE_URL=` **explicitly** (unset ≠ direct); use `RELAY_EDGE_DIRECT=1` deploy |
| Edge left in direct mode after direct tests | Redeploy **without** `RELAY_EDGE_DIRECT` before gateway e2e |
| Relay blip mid long matrix (`:8443` down) | Wait for healthz; re-run `e2e-direct-relay.sh` |
| relay-edge unreachable on `:18086` | `./scripts/deploy-remote.sh <HOST>` |
| Forge ack curl SSL error | Scripts use `RELAY_TLS_INSECURE=1` / `curl -k` for Relay HTTPS |
| Policy patch JSON error | Fetch policy via `GET /v1/policies` list (no get-by-id route) |

---

## Reproduce on any lab

```bash
# 0) If Farm Act fails with TLS unknown authority / mock targets:
#    RELAY_BIN=/path/to/linux-amd64-relay ./scripts/lab-wire-relay-act.sh <HOST>
#    then re-sync pubsub RELAY_AUTH_TOKEN and restart relay-pubsub

export BASE=https://<relay-host>:8443
export GATEWAY=https://<gateway-host>:8081
export EDGE=http://<edge-host>:18086
export FORGE_BASE=http://<forge-host>:30631   # optional
export FORGE_API_KEY=<secret>                 # optional
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1

./scripts/stack-probe.sh --forge-optional
./scripts/e2e-stack.sh

# Direct (no pubsub) — then restore gateway deploy without RELAY_EDGE_DIRECT
RELAY_EDGE_DIRECT=1 RELAY_AUTH_TOKEN=<jwt> ./scripts/deploy-remote.sh <HOST>
./scripts/e2e-direct-stack.sh
RELAY_AUTH_TOKEN=<jwt> ./scripts/deploy-remote.sh <HOST>
```

Unit / smoke tests (no Relay required for smoke; unit always):

```bash
go test ./...
EDGE=http://127.0.0.1:18086 ./scripts/smoke.sh
EDGE=http://127.0.0.1:18086 ./scripts/smoke-firewater.sh
EDGE=http://127.0.0.1:18086 ./scripts/smoke-remote-edge.sh
```

---

## Related commits

| Repo | Commit | Change |
|------|--------|--------|
| relay-edge | [b206e8f](https://github.com/zyvorai/relay-edge/commit/b206e8f) | `lab-wire-relay-act.sh`; document full PASS after Act TLS fix |
| relay-edge | [1247041](https://github.com/zyvorai/relay-edge/commit/1247041) | `e2e-direct-stack.sh`, drone_patrol in gateway matrix, gateway env tests |
| relay-edge | [70e4742](https://github.com/zyvorai/relay-edge/commit/70e4742) | Direct Relay e2e script, expanded scenarios, `GATEWAY_BASE_URL=` fix |
| relay | [8bef494](https://github.com/zyvorai/relay/commit/8bef494) | `RELAY_TLS_INSECURE` for outbound action gateway |

---

## Related documentation

| Topic | Document |
|-------|----------|
| Simulate all one-liner | [INTEGRATION.md — Simulate all](INTEGRATION.md#simulate-all-one-command) |
| Direct Relay path | [RELAY.md — Try direct mode](RELAY.md#try-direct-mode-locally) |
| Event type tables | [EVENT_MATRIX.md](EVENT_MATRIX.md) |
| Configuration / empty gateway | [CONFIGURATION.md](CONFIGURATION.md) |
| Lab ports & deploy | [DEPLOYMENT.md](DEPLOYMENT.md) |
| Browser summary | [/ui/docs.html](/ui/docs.html) |
