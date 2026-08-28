# Getting started

Five minutes from clone to a running control room with simulated events.

← [Docs hub](README.md)

---

## Prerequisites

- Go 1.23+
- `curl`, `python3` (for smoke scripts)

Optional for full integration testing:

- [relay-pubsub](https://github.com/zyvorai/relay-pubsub) running with `RELAY_BACKEND=relay-events`
- Zyvor Relay reachable at `RELAY_BASE_URL`

---

## 1. Run the server

```bash
git clone https://github.com/zyvorai/relay-edge.git
cd relay-edge
go test ./...
go run ./cmd/relay-edge
```

Open **http://127.0.0.1:18086/ui** — the fire-water control room.

---

## 2. Farm smoke (API-only)

In another terminal:

```bash
./scripts/smoke.sh
```

This walks the full farm lifecycle: site → zone → device → season → open → stage → advisory → critical irrigation event.

---

## 3. Industrial simulators

**Fire-water plant**

```bash
./scripts/smoke-firewater.sh
# or interactively: Seed → scenarios in /ui
```

**Remote edge NOC**

```bash
./scripts/smoke-remote-edge.sh
# or open http://127.0.0.1:18086/ui/remote-edge.html
```

**Master fleet catalog**

Open http://127.0.0.1:18086/ui/fleet.html and try scenarios like `blackout` or `amr_lost`.

---

## 4. Publish into Relay (optional)

When relay-pubsub and Relay are running:

```bash
# Seed shared industrial season
curl -fsS -X POST http://127.0.0.1:18086/v1/firewater/seed

# Enable publish on firewater
curl -fsS -X POST http://127.0.0.1:18086/v1/firewater/config \
  -H 'content-type: application/json' \
  -d '{"publish":true,"interval_ms":5000}'

# Trigger a scenario — events flow: edge → gateway → Relay
curl -fsS -X POST http://127.0.0.1:18086/v1/firewater/scenario \
  -H 'content-type: application/json' \
  -d '{"scenario":"lowtank"}'
```

Set environment before starting edge:

```bash
export GATEWAY_BASE_URL=https://127.0.0.1:8081
export RELAY_AUTH_TOKEN=<your-jwt>
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
```

**Remote edge / Fleet** — same workflow in the browser:

1. Open `/ui/remote-edge.html` or `/ui/fleet.html`
2. **Seed plant inventory** (shared with firewater)
3. Enable **Publish into Relay** → **Apply config**
4. Pick a scenario or **Start stream**

Same pattern for remote-edge and fleet in the UI, or via curl (`POST /v1/remote-edge/config`, `POST /v1/fleet/config`).

### Direct to Relay (no relay-pubsub)

Skip the gateway — edge POSTs straight to Relay's `/v1/events`:

```bash
export GATEWAY_BASE_URL=
export RELAY_BASE_URL=https://127.0.0.1:8443
export RELAY_AUTH_TOKEN=<your-jwt>
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
./scripts/smoke.sh   # publish.path should be "relay"
```

Full details → [Working with Relay](RELAY.md)

---

## 5. HTTPS mode (pods / production-style)

```bash
export EDGE_TLS=1
export EDGE_TLS_SAN=localhost,127.0.0.1
go run ./cmd/relay-edge
curl -k https://127.0.0.1:18086/healthz
```

Certs are generated once under `/var/lib/relay-edge/tls/` (or local `./data` parent paths in dev).

---

## 6. Forge + Relay (optional)

When **[Forge](https://github.com/zyvorai/forge)** runs at the same site, Relay can gate critical acts behind Forge **Decision Records** (`decision_backend: forge` on policy). relay-edge only publishes events — approval happens in Relay + Forge.

```bash
# On Relay (not relay-edge):
RELAY_FORGE_BASE_URL=http://<forge-host>:30631
RELAY_FORGE_API_KEY=<forge-api-gateway-secret>
```

Try: farm critical event from `./scripts/smoke.sh` with a forge-backed policy, then freeze/attest in Forge Zeus.

→ [Forge at the edge](FORGE.md) · [forge RELAY_STACK](https://github.com/zyvorai/forge/blob/main/docs/integrations/RELAY_STACK.md)

---

## Next steps

- [Concepts](CONCEPTS.md) — why edge owns the domain, how stamping works
- [Working with Relay](RELAY.md) — direct vs gateway, wire format, lifecycle
- [Forge at the edge](FORGE.md) — Forge + relay-edge + decision-making
- [Simulators](SIMULATORS.md) — scenarios, event types, UIs
- [Event matrix](EVENT_MATRIX.md) — verify all four families end-to-end
- [Deployment](DEPLOYMENT.md) — systemd or Kubernetes
