# Stack test results

Live verification of relay-edge + relay-pubsub + Relay (+ Forge) on lab host **`212.8.248.187`**.

← [Docs hub](README.md) · [Integration guide](INTEGRATION.md) · [Event matrix](EVENT_MATRIX.md) · [Browser docs](/ui/docs.html)

**Last run:** 2026-08-28 · **Outcome:** all gates **PASS**

---

## What we tested

End-to-end proof that stamped events from relay-edge reach Relay through relay-pubsub, policies fire correctly, actions execute via the pubsub Action Gateway, and optional Forge Decision Records gate critical acts.

| Layer | Verified behaviour |
|-------|-------------------|
| **relay-edge** | Farm lifecycle publish, firewater / remote-edge / fleet scenarios with `publish:true` |
| **relay-pubsub** | Pub/Sub REST publish → Relay `relay-events`; inbound `POST /v1/actions` |
| **Relay** | Accept, notify, ack, act, verify; Forge `awaiting_decision` when policy uses `decision_backend: forge` |
| **Forge** | Decision Record create (Relay), human freeze Approved → act; Rejected → failed |

---

## Lab topology

| Service | URL | Repo |
|---------|-----|------|
| relay-edge | `http://212.8.248.187:18086` | relay-edge |
| relay-pubsub | `https://212.8.248.187:8081` | relay-pubsub |
| Relay | `https://212.8.248.187:8443` | relay |
| Forge gateway | `http://212.8.248.187:30631` | forge |
| Forge UI (Zeus) | `http://212.8.248.187:30862` | forge |

Relay process env (lab):

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

---

## How we tested

### 1. Prepare environment

```bash
git clone https://github.com/zyvorai/relay-edge.git && cd relay-edge
cp config/lab-stack.env.example config/lab-stack.env
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
RELAY_AUTH_TOKEN=$TOKEN ./scripts/deploy-remote.sh 212.8.248.187
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
| **C. Remote edge** | 5 scenarios | New Relay event per type (`remote-edge.*`) |
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

## Results (2026-08-28)

### stack-probe.sh

```
  ok  relay-edge http://212.8.248.187:18086/healthz
  ok  relay-pubsub https://212.8.248.187:8081/healthz
  ok  Relay https://212.8.248.187:8443/healthz
  ok  Forge http://212.8.248.187:30631/api/zeus/decisions
PASS: stack probe
```

### e2e-events-matrix.sh

| Section | Result |
|---------|--------|
| A. Farm catalog | **10/10** Accept; **5/5** Act via `rpg_*` |
| B. Firewater / edge | **5/5** events in Relay |
| C. Remote edge | **5/5** events in Relay |
| D. Fleet | **6/6** events in Relay |

**Exit code:** 0

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
| Gateway `relay 401 Unauthorized` | Re-sync `RELAY_AUTH_TOKEN` on pubsub after Relay restart |
| Farm Act `failed` — TLS cert verify | Set `RELAY_TLS_INSECURE=1` on Relay ([relay#8bef494](https://github.com/zyvorai/relay/commit/8bef494)) |
| Action circuit breaker open | Reset by fixing TLS; publish fresh catalog events |
| relay-edge unreachable on `:18086` | `./scripts/deploy-remote.sh <HOST>` |
| Forge ack curl SSL error | Scripts use `RELAY_TLS_INSECURE=1` / `curl -k` for Relay HTTPS |
| Policy patch JSON error | Fetch policy via `GET /v1/policies` list (no get-by-id route) |

---

## Reproduce locally against any lab

```bash
export BASE=https://<relay>:8443
export GATEWAY=https://<gateway>:8081
export EDGE=http://<edge>:18086
export FORGE_BASE=http://<forge>:30631
export FORGE_API_KEY=<secret>
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1

./scripts/stack-probe.sh
./scripts/e2e-events-matrix.sh
./scripts/e2e-forge-stack.sh
```

Unit / smoke tests (no Relay required):

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
| relay-edge | [d9c9958](https://github.com/zyvorai/relay-edge/commit/d9c9958) | e2e script fixes, lab troubleshooting docs |
| relay | [8bef494](https://github.com/zyvorai/relay/commit/8bef494) | `RELAY_TLS_INSECURE` for outbound action gateway |

---

## Related documentation

| Topic | Document |
|-------|----------|
| Simulate all one-liner | [INTEGRATION.md — Simulate all](INTEGRATION.md#simulate-all-one-command) |
| Event type tables | [EVENT_MATRIX.md](EVENT_MATRIX.md) |
| Lab ports & deploy | [DEPLOYMENT.md](DEPLOYMENT.md) |
| Browser summary | [/ui/docs.html](/ui/docs.html) |
