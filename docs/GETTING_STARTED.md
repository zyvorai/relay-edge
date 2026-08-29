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

**Remote edge / Fleet** — seed once via firewater, then use the browser:

1. Run `POST /v1/firewater/seed` (firewater UI **Seed plant inventory**, or curl below)
2. Open `/ui/remote-edge.html` or `/ui/fleet.html`
3. Enable **Publish into Relay** → **Apply config**
4. Pick a scenario or **Start stream**

```bash
curl -fsS -X POST http://127.0.0.1:18086/v1/firewater/seed
```

Same pattern via curl: `POST /v1/remote-edge/config`, `POST /v1/fleet/config`.

### Direct to Relay (no relay-pubsub)

Skip the gateway — edge POSTs straight to Relay's `/v1/events`:

```bash
export GATEWAY_BASE_URL=          # must be explicit empty — unset uses gateway default
export RELAY_BASE_URL=https://127.0.0.1:8443
export RELAY_AUTH_TOKEN=<your-jwt>
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
./scripts/smoke.sh   # publish.path should be "relay"
```

Full expanded matrix (farm + all simulator scenarios):

```bash
cp config/lab-direct.env.example config/lab-direct.env
# BASE, EDGE, RELAY_AUTH_TOKEN

set -a && source config/lab-direct.env && set +a
./scripts/e2e-direct-stack.sh    # probe + e2e-direct-relay.sh
```

Remote deploy in direct mode: `RELAY_EDGE_DIRECT=1 RELAY_AUTH_TOKEN=<jwt> ./scripts/deploy-remote.sh <HOST>`

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

## 6. Full stack simulation (Forge optional)

When **[Forge](https://github.com/zyvorai/forge)** runs at the same site, Relay can gate critical acts behind Forge **Decision Records**. relay-edge only publishes events — **Forge is not required** for the core stack.

**No Forge** (relay-edge + pubsub + Relay):

```bash
cp config/lab-stack.env.example config/lab-stack.env
# BASE, GATEWAY, EDGE, RELAY_AUTH_TOKEN — leave FORGE_* empty

set -a && source config/lab-stack.env && set +a
./scripts/e2e-stack.sh
```

**With Forge** — add `FORGE_BASE`, `FORGE_API_KEY`, and `RELAY_FORGE_*` on Relay, then `./scripts/e2e-forge-stack.sh`.

**Farm Act fails (TLS / mock targets)?** Wire Relay first:

```bash
RELAY_BIN=/path/to/linux-amd64-relay ./scripts/lab-wire-relay-act.sh <HOST>
# then sync pubsub JWT and ./scripts/e2e-stack.sh
```

→ [Integration guide](INTEGRATION.md#simulate-all-one-command) · [Test results](TEST_RESULTS.md)

---

## Next steps

- [Concepts](CONCEPTS.md) — why edge owns the domain, how stamping works
- [Working with Relay](RELAY.md) — direct vs gateway, wire format, lifecycle
- [Integration guide](INTEGRATION.md) — relay-edge + Relay + Forge, simulate all
- [Simulators](SIMULATORS.md) — scenarios, event types, UIs
- [Event matrix](EVENT_MATRIX.md) — verify all four families end-to-end (gateway or direct)
- [Direct Relay test](RELAY.md#try-direct-mode-locally) — `e2e-direct-relay.sh` without pubsub
- [Deployment](DEPLOYMENT.md) — systemd or Kubernetes
