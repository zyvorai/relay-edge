# Configuration

Environment variables read by `cmd/relay-edge/main.go` and the publish client in `internal/relaypub/client.go`.

← [Docs hub](README.md)

---

## Server

| Variable | Default | Purpose |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen address (HTTP or HTTPS when TLS enabled) |
| `EDGE_DATA_DIR` | `./data` | Directory for JSON stores (`seasons.json`, `sites.json`, `zones.json`, `devices.json`, `contacts.json`) |
| `EDGE_TLS` | `0` | `1` = serve API + UIs over self-signed HTTPS |
| `EDGE_TLS_CERT` | `/var/lib/relay-edge/tls/cert.pem` | TLS certificate path (generated on first run if missing) |
| `EDGE_TLS_KEY` | `/var/lib/relay-edge/tls/key.pem` | TLS private key path |
| `EDGE_TLS_SAN` | `localhost,relay-edge` | Comma-separated SANs for generated cert |

Bool parsing: `1` / `true` / `yes` / `on` → true; `0` / `false` / `no` / `off` → false.

---

## Publish path (Relay / relay-pubsub)

| Variable | Default | Purpose |
|----------|---------|---------|
| `GATEWAY_BASE_URL` | `https://127.0.0.1:8081` | relay-pubsub base URL. **Non-empty → gateway path.** Set to empty string for direct Relay. |
| `GATEWAY_AUTH_TOKEN` | — | Optional Bearer JWT for gateway |
| `RELAY_BASE_URL` | `https://127.0.0.1:18080` | Relay `POST /v1/events` (direct path only). Lab/production Relay is usually `:8443`. |
| `RELAY_AUTH_TOKEN` | — | Bearer JWT for Relay (and often shared with relay-pubsub) |
| `RELAY_TLS_INSECURE` | `1` | Skip TLS certificate verification for Relay/gateway (lab self-signed) |
| `FASAL_GCP_PROJECT` | `fasal-onprem` | GCP project segment in gateway publish URL: `…/projects/{project}/topics/{type}:publish` |

### Which path is used?

```text
GATEWAY_BASE_URL set (default)  →  POST …/topics/{eventType}:publish  →  path: "gateway"
GATEWAY_BASE_URL empty          →  POST {RELAY_BASE_URL}/v1/events     →  path: "relay"
```

Startup log line shows the active URLs:

```text
relay-edge listening on http://:18086 (data=./data gateway=https://127.0.0.1:8081 relay=https://127.0.0.1:18080 tls=false)
```

---

## Typical profiles

### Local simulators only (no Relay)

No env vars required. Publish toggles stay `false` in the UIs.

### Lab stack (relay-pubsub + Relay on :8443)

```bash
export GATEWAY_BASE_URL=https://127.0.0.1:8081
export RELAY_BASE_URL=https://127.0.0.1:8443   # used if gateway empty; also logged at startup
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1
```

### Direct to Relay (no relay-pubsub)

```bash
export GATEWAY_BASE_URL=
export RELAY_BASE_URL=https://127.0.0.1:8443
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
| `BASE` | — | Relay URL for integration scripts |
| `GATEWAY` | — | relay-pubsub URL for integration scripts |
| `EDGE` | `http://127.0.0.1:18086` | relay-edge URL for smoke/e2e |
| `PROJECT` | `fasal-onprem` | Gateway project in e2e matrix |
| `RELAY_DEMO_USER` / `RELAY_DEMO_PASSWORD` | `demo` / `demo` | JWT login in stack-probe |
| `FORGE_BASE` / `FORGE_API_KEY` | — | Optional Forge checks in e2e-forge-stack |
| `REMOTE_DIR` / `EDGE_PORT` | — | deploy-remote.sh |

See `config/lab-stack.env.example` for a copy-paste lab template.

---

## Related

- [Working with Relay](RELAY.md) — wire format, stamped fields, Act targets
- [Deployment](DEPLOYMENT.md) — systemd and Kubernetes defaults
- [Getting started](GETTING_STARTED.md) — first run with optional publish
