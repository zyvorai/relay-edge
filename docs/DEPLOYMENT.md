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
| `GATEWAY_BASE_URL` | Peer pubsub URL (remote OK; omit when `RELAY_EDGE_DIRECT=1`) |
| `RELAY_BASE_URL` | Peer Relay URL (`:8443` / `:18080`) — not laptop localhost |
| `RELAY_TLS_INSECURE` | `1` |
| `EDGE_TLS` | `1` (HTTPS; accept browser warning) |

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
EDGE=https://<HOST>:18086 ./scripts/smoke.sh
EDGE=https://<HOST>:18086 ./scripts/smoke-firewater.sh
EDGE=https://<HOST>:18086 ./scripts/smoke-remote-edge.sh
EDGE=https://<HOST>:18086 ./scripts/smoke-fleet.sh
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

Default Helm image: `ghcr.io/zyvorai/relay-edge` (published on every `v*` release and on main via `release-image`). Lab script builds locally with **podman** (or `BUILDER=docker`) and imports into k3s — it does not require GHCR pull.

```bash
# Local image build (also what deploy-k8s-remote.sh does on the host)
BUILDER=podman ./deploy/scripts/deploy-k8s-remote.sh <HOST>

# Or pull a published image into your own cluster:
helm upgrade --install relay-edge deploy/helm/relay-edge \
  --set image.repository=ghcr.io/zyvorai/relay-edge \
  --set image.tag=0.1.1
```

Optional Helm values: `edge.enabledFamilies` (`EDGE_ENABLED_FAMILIES`), `edge.gatewayAuthTokenKey`, `edge.apiTokenKey` (`EDGE_API_TOKEN`), `edge.requireAuth`, `tls.existingSecret`, `ingress.*`. Production starting point: `values-production.yaml` · checklist: [PRODUCTION.md](PRODUCTION.md).

Backup / restore: `./scripts/backup-data.sh` · `./scripts/restore-data.sh`.

---

## Configuration essentials

| Variable | Default | Purpose |
|----------|---------|---------|
| `EDGE_HTTP_ADDR` | `:18086` | Listen |
| `EDGE_TLS` | `0` | `1` = HTTPS with auto cert |
| `EDGE_TLS_CERT` / `EDGE_TLS_KEY` | `/var/lib/relay-edge/tls/*.pem` | Cert paths |
| `EDGE_TLS_SAN` | `localhost,relay-edge` | SAN for generated cert |
| `EDGE_API_TOKEN` | — | Bearer for `/v1/*` |
| `EDGE_REQUIRE_AUTH` | `0` | Fail start without token |
| `EDGE_ENABLED_FAMILIES` | _(all)_ | Optional subset: `firewater`, `remote-edge`, `fleet` |
| `EDGE_DATA_DIR` | `./data` | JSON stores |
| `GATEWAY_BASE_URL` | Peer pubsub URL | Empty = direct; remote pubsub OK |
| `RELAY_BASE_URL` | Peer Relay URL | Direct path; remote Relay OK (`:8443` / `:18080`) |
| `RELAY_AUTH_TOKEN` | — | JWT (sync with pubsub) |
| `RELAY_TLS_INSECURE` | `1` | Skip TLS verify outbound |
| `FASAL_GCP_PROJECT` | `fasal-onprem` | Gateway project id |

---

## Lab reference

**Any of the three projects can be remote.** Relay, relay-pubsub, and relay-edge do not have to share a host. Wire each process to the **reachable URL** of its peer; run e2e from your workstation with `BASE` / `GATEWAY` / `EDGE` set to those same remote URLs.

| Who talks to whom | Use |
|-------------------|-----|
| Your laptop → stack (`e2e-stack.sh`, curl, browser) | Public / lab host IPs — **never** `127.0.0.1` (that is your laptop) |
| edge → pubsub (`GATEWAY_BASE_URL`) | Host where pubsub listens (remote OK) |
| edge → Relay direct (`RELAY_BASE_URL`) | Host where Relay listens (remote OK) |
| pubsub → Relay (`RELAY_BASE_URL`) | Host where Relay listens (remote OK) |
| Relay → Action Gateway (`RELAY_ACTION_TARGETS`) | Host where pubsub `/v1/actions` listens — `127.0.0.1` **only** if Relay and pubsub are on the same machine |

Example all-on-one labs (still use the host IP from your laptop):

| Host | Relay | pubsub | console | edge |
|------|-------|--------|---------|------|
| `<ephemeral-ip>` | `:8443` | `:8081` | `:8082` | `:18086` |
| `<ephemeral-ip>` | `:18080` | `:8081` | `:8082` | `:18086` |

Env templates: [`config/lab-stack.env.example`](../config/lab-stack.env.example) · [`config/lab-stack-175.env.example`](../config/lab-stack-175.env.example).

| Service | Port | Notes |
|---------|------|-------|
| relay-edge | 18086 | HTTPS on remote deploy (`EDGE_TLS=1`) |
| relay-pubsub | 8081 | systemd; 8080 in-cluster |
| relay-pubsub console | 8082 | Stored UI (proxies gateway) |
| Relay | 8443 or 18080 | Host process — check `RELAY_PORT` / `RELAY_ADDR` |
| Forge Web UI | 30862 | Optional — sibling [forge](https://github.com/zyvorai/forge) repo |
| Forge API gateway | 30631 | Relay `RELAY_FORGE_BASE_URL` for Decision Records |

**JWT:** same token in edge env, pubsub env, and k8s secrets (wherever each runs). After every Relay restart, re-login (`demo`/`demo`) and re-sync.

**Farm Act:** Relay `RELAY_ACTION_TARGETS` must reach pubsub’s `/v1/actions` (remote host URL if pubsub is not local). Binary must honor `RELAY_TLS_INSECURE=1` for self-signed HTTPS ([relay#8bef494](https://github.com/zyvorai/relay/commit/8bef494)). Gateway cert SAN must include every name/IP Relay uses to call it. Helper: [`scripts/lab-wire-relay-act.sh`](../scripts/lab-wire-relay-act.sh) (assumes co-located Act targets unless you edit them).

**Forge decisions (optional):** configure `RELAY_FORGE_*` on Relay only — not on relay-edge.

**Stack verification:** from your workstation, `./scripts/e2e-stack.sh` with remote `BASE`/`GATEWAY`/`EDGE` — see [TEST_RESULTS.md](TEST_RESULTS.md).

---

## Scripts reference

| Script | Purpose |
|--------|---------|
| `scripts/deploy-remote.sh` | Build + SSH deploy (systemd or nohup; `RELAY_EDGE_DIRECT=1` for direct) |
| `scripts/lab-wire-relay-act.sh` | Wire Relay Act targets + TLS insecure on lab |
| `deploy/scripts/deploy-k8s-remote.sh` | Full k8s stack (`BUILDER=podman` default) |
| `deploy/scripts/k8s-e2e.sh` | Port-forward + smoke on cluster |
| `scripts/smoke*.sh` | Local farm / firewater / remote-edge / fleet smokes |
| `scripts/e2e-events-matrix.sh` | All families → Relay (gateway) |
| `scripts/e2e-direct-stack.sh` | Direct Relay probe + expanded matrix |
| `Makefile` | `make vet test build smoke-all release-binaries` |

## CI and releases

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| **CI** | PR + push to `main` | `go vet` / `go test` + four smokes vs mock Relay |
| **Release** | `v*` tag or Actions → Run workflow | Multi-arch binaries + GitHub Release + GHCR image |
| **release-image** | Push to `main` (code paths) | `ghcr.io/zyvorai/relay-edge:latest` + `sha-*` |

Latest: [releases](https://github.com/zyvorai/relay-edge/releases) · image `ghcr.io/zyvorai/relay-edge`.
