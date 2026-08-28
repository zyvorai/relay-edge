# Forge at the edge

How **[Forge](https://github.com/zyvorai/forge)** fits with relay-edge and Relay at remote sites, factory floors, and GPU clusters.

← [Docs hub](README.md) · Forge’s view: [forge RELAY_STACK](https://github.com/zyvorai/forge/blob/main/docs/integrations/RELAY_STACK.md) · Relay approval backends: [relay ARCHITECTURE](https://github.com/zyvorai/relay/blob/main/docs/ARCHITECTURE.md#approval-backends-native-vs-forge)

---

## Two different “edges”

| Term | Repo | What it is |
|------|------|------------|
| **Forge edge** | forge | A member cluster, factory GPU rack, or remote site where Forge runs AI/Kubernetes workloads |
| **relay-edge** | this repo | Farm/industrial **event companion** — stamps context and publishes into Relay |

They often run on the **same host** but solve different problems:

- **Forge** — GPUs, federated training, database cutover, Zeus copilot, federation dispatch.
- **relay-edge** — seasons/sites/zones, simulators, stamped events, `/ui` control rooms.
- **Relay** — durable Accept → Notify → Ack → Act → Verify loop.
- **relay-pubsub** (optional) — Google Pub/Sub wire at the edge.

```text
 ┌──────────────────────────────────────────────────────────────────────┐
 │  FORGE EDGE SITE (member cluster / factory / remote GPU rack)         │
 │                                                                       │
 │  Forge ──► FabricAIJob · FabricFederatedTrainingRun · Zeus           │
 │            FabricDatabaseMigration · API gateway :30631               │
 │                                                                       │
 │  relay-edge :18086 ──► farm domain + firewater / remote-edge / fleet  │
 └───────────────────────────────┬──────────────────────────────────────┘
                                 │ stamped events
                                 ▼
                    relay-pubsub (optional) ──► Zyvor Relay
                                 │
                                 ▼
                    optional: Forge Decision Records (human gate before Act)
```

Forge and relay-edge **do not share a process**. They correlate by **site identity** in event `data` (labels, `sim_domain`, federation cluster name in your integration layer).

---

## What each layer owns

| Layer | Owns | Does not own |
|-------|------|--------------|
| **Forge** | K8s AI infra, GPU placement, federated LoRA, Zeus, Decision Records | Farm seasons, notify/ack/act workers, irrigation idempotency |
| **relay-edge** | Topology, stamping, simulators, `/ui` | Durable event log, policies, actuation |
| **relay-pubsub** | Pub/Sub → Relay `/v1/events`, Action Gateway inbound | Crop topology, Forge CRDs |
| **Relay** | Reliability loop, policies, audit | Device inventory, GPU scheduling, Zeus chat |

relay-edge makes events **site-aware** before Relay sees them. Relay policies match on **type + severity**.

---

## What Forge does at the edge (sibling repo)

These Forge features naturally live at remote or factory sites:

| Capability | CRD / surface | Edge role |
|------------|---------------|-----------|
| **Local AI workloads** | `FabricAIJob`, workspaces | Train/infer where data lives |
| **Federated training** | `FabricFederatedTrainingRun` | LoRA on each member; deltas only cross the wire |
| **DB cutover** | `FabricDatabaseMigration` | Cloud RDS → local CloudNativePG; approval-gated cutover |
| **Multi-cluster** | `FabricFederation` | Registry of sites Forge dispatches to |
| **North-south API** | API gateway | Auth, tenancy — **Relay calls Forge here for decisions** |

Promotion of a merged federated adapter can itself be gated by a **FabricDecisionRecord** (human approval in Zeus).

---

## What relay-edge models (this repo)

Three browser control rooms mirror problems Forge edge sites face:

| UI | Models | Example events |
|----|--------|----------------|
| [/ui](../web/index.html) | Industrial fire-water + edge AI/comms | `firewater.tank.low`, `edge.comms.down` |
| [/ui/remote-edge.html](../web/remote-edge.html) | Remote NOC — satellite, compute rack, UAV, vision | `remote-edge.link.offline`, `remote-edge.galleon.thermal` |
| [/ui/fleet.html](../web/fleet.html) | AMR, energy, OT, building, marine | `fleet.power.island`, `fleet.robot.lost` |

All three: **Seed → Publish into Relay → Start stream → Scenarios**.

Operational story map (Forge site + relay-edge demo):

| Story | Simulator | Event type |
|-------|-----------|------------|
| Remote link down | Remote edge · `offline` | `remote-edge.link.offline` |
| Starlink degraded | Remote edge · `sat_down` | `remote-edge.link.starlink.degraded` |
| GPU thermal at rack | Remote edge · `gpu_hot` | `remote-edge.galleon.thermal` |
| Site power island | Fleet · `blackout` | `fleet.power.island` |
| AMR lost in factory | Fleet · `amr_lost` | `fleet.robot.lost` |
| Critical irrigation | Farm API / smoke | `irrigation.required` |

Full list → [Event matrix](EVENT_MATRIX.md).

---

## Decision-making: where Forge participates

relay-edge **does not** implement approval logic. It publishes stamped events; **Relay** runs the loop.

### Path A — Native (default)

```text
relay-edge event → Relay Accept → Notify → operator Approve in Relay → Act → Verify
```

No Forge dependency.

### Path B — Forge Decision Records (`decision_backend: forge`)

Use when policy needs a **durable, attestable** human decision (compliance, EHS, federated-training promotion):

```text
relay-edge critical event (e.g. irrigation.required)
  → Relay Accept (policy: require_approval + decision_backend: forge)
  → Relay POST Forge /api/zeus/decisions
  → Human in Forge Zeus: research → freeze → attest (approve / reject)
  → Relay poll_forge_decision job
  → If Frozen + Approved → Relay Act via Action Gateway
  → Verify (stamped verification_probe from relay-edge zone telemetry)
```

| Role | Who |
|------|-----|
| **Recommends** | Zeus / AI (evidence in Decision Record) |
| **Decides (human)** | Operator freeze + attest in Forge |
| **Executes** | Relay Action Gateway only |
| **Never acts** | Forge — no `irrigation.start`, `pump.start`, cordon from Forge in this path |

Relay policy snippet:

```json
{
  "require_approval": true,
  "decision_backend": "forge"
}
```

Configure on **Relay** (not relay-edge):

```bash
RELAY_FORGE_BASE_URL=http://<forge-host>:30631
RELAY_FORGE_API_KEY=<forge-api-gateway-secret>
```

If `decision_backend=forge` and Forge is unreachable, approval **fails closed** — no act.

Forge API routes Relay uses:

| Method | Route | Purpose |
|--------|-------|---------|
| `POST` | `/api/zeus/decisions` | Open Decision Record |
| `GET` | `/api/zeus/decisions/{id}` | Poll phase / decision |
| `POST` | `/api/zeus/decisions/{id}/freeze` | Human freeze |
| `POST` | `/api/zeus/decisions/{id}/attest` | Role-gated attestation |

---

## End-to-end flows

### Flow 1 — Simulator → Relay (no Forge)

Good for demos and [Event matrix](EVENT_MATRIX.md):

```bash
# relay-edge
export GATEWAY_BASE_URL=https://127.0.0.1:8081
export RELAY_AUTH_TOKEN=<jwt>
go run ./cmd/relay-edge
# UI: Seed → Publish → scenario
```

### Flow 2 — Critical event → Forge approval → Act

1. relay-edge publishes `irrigation.required` (farm smoke or API).
2. Relay policy has `decision_backend: forge`.
3. Operator approves in Relay → Forge Decision Record opens.
4. Human freezes/attests in Forge Zeus.
5. Relay acts via Action Gateway; verify hits stamped probe.

Test script (Relay repo): `scripts/decision-backend-scenarios.sh`.

### Flow 3 — Forge training + relay-edge ops events

```text
FabricFederatedTrainingRun job on edge-a fails
  (Forge federation recovery phases)

In parallel, relay-edge or monitoring emits:
  remote-edge.link.offline · fleet.robot.lost · edge.comms.down
  → Relay escalates to on-call / Forge War Room context
```

---

## Co-deployed lab (same host)

Primary lab `212.8.248.187` often runs all four services:

| Service | Typical port | Repo |
|---------|--------------|------|
| Forge Web UI | `http://212.8.248.187:30862` | forge |
| Forge API gateway | `http://212.8.248.187:30631` | forge |
| Relay HTTPS | `https://212.8.248.187:8443` | relay |
| relay-pubsub HTTPS | `https://212.8.248.187:8081` | relay-pubsub |
| relay-edge HTTP | `http://212.8.248.187:18086` | relay-edge |

### Wiring checklist

1. **Shared JWT** — `RELAY_AUTH_TOKEN` on relay-edge and relay-pubsub.
2. **Gateway** — relay-edge `GATEWAY_BASE_URL=https://127.0.0.1:8081` (or host IP from pods).
3. **Action targets** — on Relay:
   ```bash
   RELAY_ACTION_TARGETS=farm-controller=https://127.0.0.1:8081/v1/actions,\
   firewater-controller=https://127.0.0.1:8081/v1/actions,\
   remote-edge-controller=https://127.0.0.1:8081/v1/actions,\
   fleet-controller=https://127.0.0.1:8081/v1/actions
   ```
4. **Forge decisions** (optional, on Relay):
   ```bash
   RELAY_FORGE_BASE_URL=http://127.0.0.1:30631
   RELAY_FORGE_API_KEY=$(kubectl -n forge get secret forge-api-gateway-secret \
     -o jsonpath='{.data.api-key}' | base64 -d)
   ```
5. **Seed relay-edge** — `POST /v1/firewater/seed` before simulator publish.

---

## When to use what

| Goal | Use |
|------|-----|
| Run GPU training/inference at edge | **Forge** |
| Federated fine-tune across factory sites | **Forge** `FabricFederatedTrainingRun` |
| Migrate cloud DB to edge Postgres | **Forge** `FabricDatabaseMigration` |
| Human-gated decisions with audit trail | **Forge** Decision Records + Relay `decision_backend: forge` |
| Farm/site event stamping + notify/act loop | **relay-edge** + **Relay** |
| Google Pub/Sub clients at edge | **relay-pubsub** + relay-edge |
| Demo remote edge without hardware | relay-edge `/ui`, `/ui/remote-edge.html`, `/ui/fleet.html` |

---

## Related documentation

| Doc | Where |
|-----|-------|
| Publish paths (direct vs gateway) | [RELAY.md](RELAY.md) |
| Stamping and simulators | [CONCEPTS.md](CONCEPTS.md) · [SIMULATORS.md](SIMULATORS.md) |
| Cross-family integration gate | [EVENT_MATRIX.md](EVENT_MATRIX.md) |
| Forge stack integration (authoritative) | [forge RELAY_STACK](https://github.com/zyvorai/forge/blob/main/docs/integrations/RELAY_STACK.md) |
| Relay approval backends | [relay ARCHITECTURE](https://github.com/zyvorai/relay/blob/main/docs/ARCHITECTURE.md#approval-backends-native-vs-forge) |
| Forge advisory vs actuation | [forge ADVISORY_LEDGER](https://github.com/zyvorai/forge/blob/main/docs/product/ADVISORY_LEDGER.md) |

---

## Summary

**Forge** runs AI infrastructure at edge sites and holds **human Decision Records** when Relay policy requires them. **relay-edge** stamps and publishes operational events from those sites. **Relay** makes events reliable and executes acts. relay-edge never calls Forge directly — the integration is **Relay → Forge → human → Relay Act**.
