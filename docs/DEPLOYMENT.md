# Deployment

How to run relay-edge on a laptop, a Linux host, or in Kubernetes — alongside relay-pubsub.

← [Docs hub](README.md) · See [Configuration](CONFIGURATION.md) for all env vars

---

## Choose your path

```text
  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
  │   Laptop    │     │ Linux host  │     │ Kubernetes  │
  │  go run     │     │  systemd    │     │  Helm pods  │
  └──────┬──────┘     └──────┬──────┘     └──────┬──────┘
         │                   │                   │
         └───────────────────┴───────────────────┘
                             │
                    optional: relay-pubsub
                             │
                             ▼
                         Zyvor Relay
```

| Path | Best for | Command |
|------|----------|---------|
| Local | Development, UIs, unit smokes | `go run ./cmd/relay-edge` |
| systemd | Lab / edge appliance | `./scripts/deploy-remote.sh HOST` |
| Kubernetes | Fleet-scale, co-located with pubsub | `./deploy/scripts/deploy-k8s-remote.sh HOST` |

---

## Local development

```bash
go test ./...
go run ./cmd/relay-edge
```

With gateway wiring:

```bash
export GATEWAY_BASE_URL=https://127.0.0.1:8081
export RELAY_AUTH_TOKEN=<jwt>
export RELAY_TLS_INSECURE=1
go run ./cmd/relay-edge
```

HTTPS mode (matches pod behaviour):

```bash
export EDGE_TLS=1
export EDGE_TLS_SAN=localhost,127.0.0.1
go run ./cmd/relay-edge
curl -k https://127.0.0.1:18086/healthz
```

---

## Linux host (systemd or nohup)

```bash
# Optional: RELAY_AUTH_TOKEN or /tmp/lab-relay.jwt
./scripts/deploy-remote.sh <HOST> [USER]
```

Installs the binary under `~/.deployments/zyvor-relay-edge` and writes `relay-edge.env`.

- **systemd** (default when `sudo -n` works): installs [`deploy/systemd/relay-edge.service`](../deploy/systemd/relay-edge.service) as `/etc/systemd/system/relay-edge.service` and `enable --now`.
- **nohup fallback:** when systemd/passwordless sudo is unavailable (set `USE_SYSTEMD=0` to force).

| Variable | Deploy default |
|----------|----------------|
| `GATEWAY_BASE_URL` | `https://127.0.0.1:8081` (omit when `RELAY_EDGE_DIRECT=1`) |
| `RELAY_BASE_URL` | `https://127.0.0.1:8443` |
| `RELAY_TLS_INSECURE` | `1` |

Manual unit install on an appliance:

```bash
sudo cp deploy/systemd/relay-edge.service /etc/systemd/system/
sudo mkdir -p /etc/relay-edge /var/lib/relay-edge
# edit /etc/relay-edge/relay-edge.env then:
sudo systemctl enable --now relay-edge
```

**Direct Relay mode** (edge → Relay, no pubsub):

```bash
RELAY_EDGE_DIRECT=1 RELAY_AUTH_TOKEN=<jwt> ./scripts/deploy-remote.sh <HOST> [USER]
```

Then run `./scripts/e2e-direct-stack.sh` with `config/lab-direct.env`. Restore gateway mode by redeploying **without** `RELAY_EDGE_DIRECT`.

Verify:

```bash
EDGE=http://<HOST>:18086 ./scripts/smoke.sh
EDGE=http://<HOST>:18086 ./scripts/smoke-firewater.sh
EDGE=http://<HOST>:18086 ./scripts/smoke-remote-edge.sh
EDGE=http://<HOST>:18086 ./scripts/smoke-fleet.sh
```

---

## Kubernetes

Requires sibling checkout of [relay-pubsub](https://github.com/zyvorai/relay-pubsub) at `../relay-pubsub`.

On the cluster host: **kubectl**, **helm**, **podman** (or docker).

```bash
RELAY_AUTH_TOKEN="$(cat /tmp/lab-relay.jwt)" \
  ./deploy/scripts/deploy-k8s-remote.sh <HOST> [USER]
```

Creates:

| Release | Namespace | Port | TLS |
|---------|-----------|------|-----|
| `relay-pubsub` | `relay-pubsub` | 8080 | self-signed |
| `relay-edge` | `relay-edge` | 18086 | self-signed |

Verify on the cluster:

```bash
bash deploy/scripts/k8s-e2e.sh
```

Helm charts: `deploy/helm/relay-edge/` · `../relay-pubsub/deploy/helm/relay-pubsub/`

Container build:

```bash
podman build -t relay-edge:latest .
```

---

## Configuration essentials

| Variable | Default | Purpose |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen |
| `EDGE_TLS` | `0` | `1` = HTTPS with auto cert |
| `EDGE_TLS_CERT` / `EDGE_TLS_KEY` | `/var/lib/relay-edge/tls/*.pem` | Cert paths |
| `EDGE_TLS_SAN` | `localhost,relay-edge` | SAN for generated cert |
| `EDGE_DATA_DIR` | `./data` | JSON stores |
| `GATEWAY_BASE_URL` | `https://127.0.0.1:8081` | pubsub gateway |
| `RELAY_BASE_URL` | `https://127.0.0.1:8443` | Relay direct |
| `RELAY_AUTH_TOKEN` | — | JWT (sync with pubsub) |
| `RELAY_TLS_INSECURE` | `1` | Skip TLS verify outbound |
| `FASAL_GCP_PROJECT` | `fasal-onprem` | Gateway project id |

---

## Lab reference

Example multi-service layout (no hardcoded IPs in repo — pass `<HOST>` to deploy scripts):

| Service | Port | Notes |
|---------|------|-------|
| relay-edge | 18086 | HTTP on host; HTTPS in k8s |
| relay-pubsub | 8081 | systemd; 8080 in-cluster |
| Relay | 8443 | Host process |
| Forge Web UI | 30862 | Optional — sibling [forge](https://github.com/zyvorai/forge) repo |
| Forge API gateway | 30631 | Relay `RELAY_FORGE_BASE_URL` for Decision Records |

Example co-deploy on one host (Relay stack + Forge). See [Integration guide](INTEGRATION.md) for full wiring and walkthroughs.

**JWT:** same token in edge env, `/etc/relay-pubsub/relay-pubsub.env`, and k8s secrets.

**Farm Act:** Relay needs all controllers on `https://127.0.0.1:8081/v1/actions` and a binary that honors `RELAY_TLS_INSECURE=1` ([relay#8bef494](https://github.com/zyvorai/relay/commit/8bef494)). Use [`scripts/lab-wire-relay-act.sh`](../scripts/lab-wire-relay-act.sh). Gateway cert SAN must include `127.0.0.1`.

**Forge decisions (optional):** configure `RELAY_FORGE_*` on Relay only — not on relay-edge.

**Stack verification:** after deploy, run `./scripts/e2e-stack.sh` or `./scripts/e2e-direct-stack.sh` — see [TEST_RESULTS.md](TEST_RESULTS.md) (2026-08-29 — all PASS).

---

## Scripts reference

| Script | Purpose |
|--------|---------|
| `scripts/deploy-remote.sh` | Build + SSH deploy to host (`RELAY_EDGE_DIRECT=1` for direct) |
| `scripts/lab-wire-relay-act.sh` | Wire Relay Act targets + TLS insecure on lab |
| `deploy/scripts/deploy-k8s-remote.sh` | Full k8s stack |
| `deploy/scripts/k8s-e2e.sh` | Port-forward + smoke on cluster |
| `scripts/e2e-events-matrix.sh` | All families → Relay (gateway) |
| `scripts/e2e-direct-stack.sh` | Direct Relay probe + expanded matrix |
