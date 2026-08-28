# Concepts

How relay-edge fits into the Zyvor stack — and why it exists as a separate companion.

← [Docs hub](README.md)

---

## The division of labor

| Layer | Owns | Does not own |
|-------|------|--------------|
| **relay-edge** | Site topology, seasons, four IoT simulators, stamping | Durable event log, notify/ack/act loop |
| **relay-pubsub** | Google Pub/Sub wire, topic → event mapping | Crop calendars, plot maps |
| **Relay** | Accept → Notify → Ack → Act → Verify | Device inventory, season timelines |
| **Forge** (optional, sibling) | GPU/AI infra at edge, Zeus, Decision Records | Event stamping, farm domain, actuation |

Relay policies match on **event type + severity**. Edge makes those events **site-aware** by stamping season, site, zone, device, recipients, and verification probes into every payload before publish — for farm, firewater, remote-edge, and fleet families alike.

When Relay policy sets `decision_backend: forge`, Forge holds the **human approval record**; Relay still executes acts.

**Default stack (no Forge):** relay-edge → relay-pubsub → Relay → Act back through pubsub `/v1/actions`. Diagrams and step-by-step → [Integration § Stack without Forge](INTEGRATION.md#stack-without-forge-default).

---

## The stamp

Before any publish, edge resolves:

```text
season → site → zone → device → contacts (via site routing)
                              → telemetry probe (via zone)
```

The result lands in the event `data` envelope Relay already understands — plus `recommended_action` for critical simulator events.

| Field | Source |
|-------|--------|
| `season_id`, `season_name`, `crop`, `stage` | Season record |
| `site`, `site_id` | Linked site |
| `zone`, `zone_id` | Resolved zone |
| `device_id`, `fasal_device_id` | Resolved device |
| `recipient`, `sms_recipient`, `email_recipient` | Site routing → contact |
| `verification_probe` | Zone telemetry (URL, method, json_path, expect) |
| `recommended_action` | `{ target, command, payload }` when command set |
| `sim_domain` | Simulator family (`firewater`, `remote-edge`, `fleet`) |

Farm critical events also require an **active season**. Simulators reuse the firewater seed season (`season_fw_watch`) so remote-edge and fleet publishes share the same industrial context — run `POST /v1/firewater/seed` first.

---

## Two publish paths

Full reference → **[Working with Relay](RELAY.md)** (direct `POST /v1/events` vs gateway, stamp format, Act wiring, examples).

```text
                    ┌─────────────────────┐
                    │   relay-edge        │
                    └──────────┬──────────┘
                               │
              ┌────────────────┴────────────────┐
              │                                 │
              ▼                                 ▼
   GATEWAY_BASE_URL set?              GATEWAY_BASE_URL empty
              │                                 │
              ▼                                 ▼
   POST …/topics/{type}:publish      POST /v1/events
   (relay-pubsub, preferred)         (Relay direct)
```

**Gateway path** is preferred in production: topic name equals event type (`irrigation.required`, `firewater.tank.low`, …). Same contract [relay-pubsub](https://github.com/zyvorai/relay-pubsub) documents for `RELAY_BACKEND=relay-events`.

**Direct path** skips relay-pubsub — set `GATEWAY_BASE_URL=` empty and point `RELAY_BASE_URL` at Relay. Useful for minimal stacks and debugging stamped payloads.

Set `RELAY_TLS_INSECURE=1` when talking to self-signed HTTPS peers (lab default).

---

## Four event families

| Family | Origin | Typical use |
|--------|--------|-------------|
| **Farm** | Season lifecycle, advisories, critical farm events | Real agronomy workflows |
| **Firewater / edge** | Industrial plant + edge AI/comms simulator | NFPA-style plant + edge fleet |
| **Remote edge** | Remote-edge NOC simulator | Starlink, Galleon, UAV, vision |
| **Fleet** | Master catalog simulator | AMR, energy, OT, building, marine, … |

All four can flow through the same gateway and hit the same Relay instance. See [Event matrix](EVENT_MATRIX.md) for the verification gate.

---

## Simulators vs farm API

Simulators are **not** a separate binary. They run inside the same process:

- Tick locally by default (SSE stream to `/ui`)
- Publish to Relay only when `"publish": true` in config
- Derive typed events from scenario state (`Derive()` in each package)

This keeps demo, test, and integration paths identical — one HTTP API, one stamp pipeline.

---

## Kubernetes mental model

Both relay-edge and relay-pubsub can run as pods with **built-in self-signed HTTPS**:

```text
  ┌─────────────────────────────────────────┐
  │  namespace: relay-edge                  │
  │  relay-edge pod → HTTPS :18086          │
  └──────────────────┬──────────────────────┘
                     │ in-cluster TLS
  ┌──────────────────▼──────────────────────┐
  │  namespace: relay-pubsub              │
  │  relay-pubsub pod → HTTPS :8080         │
  └──────────────────┬──────────────────────┘
                     │ relay-events
  ┌──────────────────▼──────────────────────┐
  │  host Relay → HTTPS :8443               │
  └─────────────────────────────────────────┘
```

Deploy both with `./deploy/scripts/deploy-k8s-remote.sh` — see [Deployment](DEPLOYMENT.md).
