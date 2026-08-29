# Configuration

Environment variables read by `cmd/relay-edge/main.go` and the publish client in `internal/relaypub/client.go`.

← [Docs hub](README.md)

---

## Server

| Variable | Default | Purpose |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen address (HTTP or HTTPS when TLS enabled) |
| `EDGE_DATA_DIR` | `./data` | Directory for JSON stores (`seasons.json`, `sites.json`, `zones.json`, `devices.json`, `contacts.json`) |
| `EDGE_TLS` | `1` | `1` = serve API + UIs over self-signed HTTPS (default) |
| `EDGE_TLS_CERT` | `{EDGE_DATA_DIR}/tls/cert.pem` | TLS certificate path (generated on first run if missing) |
| `EDGE_TLS_KEY` | `{EDGE_DATA_DIR}/tls/key.pem` | TLS private key path |
| `EDGE_TLS_SAN` | `localhost,127.0.0.1,relay-edge` | Comma-separated SANs for generated cert |
| `EDGE_API_TOKEN` | _(unset)_ | When set, require `Authorization: Bearer …` (or `X-Edge-Token` / `?token=`) for `/v1/*`. Public: health/ready/version/metrics/`/ui`. |
| `EDGE_REQUIRE_AUTH` | `0` | `1` = refuse to start if `EDGE_API_TOKEN` is empty |
| `EDGE_ENABLED_FAMILIES` | _(unset = all)_ | Comma-separated simulator families to mount: `firewater`, `remote-edge`, `fleet`. Farm routes (sites/zones/devices/contacts/seasons) always mount. Example: `fleet,firewater`. |

Bool parsing: `1` / `true` / `yes` / `on` → true; `0` / `false` / `no` / `off` → false.

---

## Publish path (Relay / relay-pubsub)

| Variable | Default | Purpose |
|----------|---------|---------|
| `GATEWAY_BASE_URL` | `https://127.0.0.1:8081` | relay-pubsub base URL (**local default only**). Use `https://<pubsub-host>:8081` when pubsub is remote. Set to **empty string** (`GATEWAY_BASE_URL=`) for direct Relay — must be explicitly set; unset uses default. |
| `GATEWAY_AUTH_TOKEN` | — | Optional Bearer JWT for gateway |
| `RELAY_BASE_URL` | `https://127.0.0.1:18080` | Relay `POST /v1/events` (direct path). Point at the host where Relay listens (`:8443` / `:18080`). **Not** laptop `127.0.0.1` unless Relay runs on the same machine as edge. |
| `RELAY_AUTH_TOKEN` | — | Bearer JWT for Relay (and often shared with relay-pubsub) |
| `RELAY_TLS_INSECURE` | `1` | Skip TLS certificate verification for Relay/gateway (lab self-signed) |
| `FASAL_GCP_PROJECT` | `fasal-onprem` | GCP project segment in gateway publish URL: `…/projects/{project}/topics/{type}:publish` |

### Which path is used?

```text
GATEWAY_BASE_URL unset          →  default https://127.0.0.1:8081  →  path: "gateway"
GATEWAY_BASE_URL=<url>          →  POST …/topics/{eventType}:publish  →  path: "gateway"
GATEWAY_BASE_URL=  (empty, set) →  POST {RELAY_BASE_URL}/v1/events     →  path: "relay"
```

**Important:** `export GATEWAY_BASE_URL=` (empty) is not the same as leaving the variable unset. Only an **explicit empty** value selects direct Relay.

Startup log line shows the active URLs:

```text
relay-edge v0.1.1 listening on http://:18086 (data=./data gateway=https://127.0.0.1:8081 relay=https://127.0.0.1:18080 tls=false)
```

`/healthz` and `/version` also report the build version (set via `-ldflags -X main.version=…`).

---

## Typical profiles

### Local simulators only (no Relay)

No env vars required. Publish toggles stay `false` in the UIs.

### Lab / remote stack (any service may be on another host)

Relay, pubsub, and edge can each be remote. Process env and e2e scripts must use **reachable host URLs** — `127.0.0.1` only when the peer truly shares that process’s machine.

```bash
# On the edge host (or in deploy env) — point at wherever pubsub/Relay run:
export GATEWAY_BASE_URL=https://<pubsub-host>:8081
export RELAY_BASE_URL=https://<relay-host>:8443   # or :18080
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1

# On your laptop — e2e hits the same remote URLs (never 127.0.0.1):
#   BASE=https://<relay-host>:…
#   GATEWAY=https://<pubsub-host>:…
#   EDGE=https://<edge-host>:…
```

### Direct to Relay (no relay-pubsub)

```bash
export GATEWAY_BASE_URL=
export RELAY_BASE_URL=https://127.0.0.1:8443   # or :18080
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1
```

### HTTPS edge (Kubernetes / production-style)

```bash
export EDGE_TLS=1
export EDGE_TLS_SAN=localhost,127.0.0.1,relay-edge
```

---

## Script-only variables

Used by `scripts/*.sh` and deploy helpers — **not** read by the relay-edge binary:

| Variable | Default | Used by |
|----------|---------|---------|
| `BASE` | — | Relay URL for integration scripts (**remote host**, not laptop localhost) |
| `GATEWAY` | — | relay-pubsub URL for integration scripts |
| `EDGE` | — | relay-edge URL for smoke/e2e (HTTPS when `EDGE_TLS=1`) |
| `PROJECT` | `fasal-onprem` | Gateway project in e2e matrix |
| `RELAY_DEMO_USER` / `RELAY_DEMO_PASSWORD` | `demo` / `demo` | JWT login in stack-probe |
| `FORGE_BASE` / `FORGE_API_KEY` | — | Optional Forge checks in e2e-forge-stack |
| `REMOTE_DIR` / `EDGE_PORT` | — | deploy-remote.sh |

See `config/lab-stack.env.example` (generic / lab 212) and `config/lab-stack-175.env.example` (lab 175, Relay `:18080`).

Direct Relay (no pubsub): `config/lab-direct.env.example` + `RELAY_EDGE_DIRECT=1` on [`deploy-remote.sh`](../scripts/deploy-remote.sh).

Farm Act on lab: [`lab-wire-relay-act.sh`](../scripts/lab-wire-relay-act.sh) sets `RELAY_ACTION_TARGETS` + `RELAY_TLS_INSECURE=1` (optional `RELAY_BIN=` for a rebuilt Relay).

---

## Related

- [Working with Relay](RELAY.md) — wire format, stamped fields, Act targets
- [Deployment](DEPLOYMENT.md) — systemd, GHCR, Kubernetes, CI/releases
- [API reference](API.md) — `/healthz`, `/readyz`, `/version`
- [Test results](TEST_RESULTS.md) — lab verification (2026-08-29)
- [Getting started](GETTING_STARTED.md) — first run with optional publish
