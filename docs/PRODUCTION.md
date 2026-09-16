---
hero:
  eyebrow: PRODUCTION
  title: Production runbook
---

User / site deployment of **relay-edge** (edge companion for Zyvor Relay).

← [Docs hub](index.md) · [Deployment](DEPLOYMENT.md) · [Security](https://github.com/zyvorai/relay-edge/blob/main/SECURITY.md)

---

## Network topology

```text
                    ┌─────────────────────────────────────────┐
  Operators / UI    │  Ingress / LB (real TLS, optional WAF)  │
  Scripts / agents │           edge.example.com              │
                    └──────────────────┬──────────────────────┘
                                       │ HTTPS
                                       ▼
                    ┌─────────────────────────────────────────┐
                    │  relay-edge :18086                       │
                    │  API auth: EDGE_API_TOKEN                │
                    │  PVC: EDGE_DATA_DIR (JSON + optional TLS)│
                    └───────────┬───────────────┬─────────────┘
                                │               │
              gateway mode      │               │  direct mode
              (recommended)     │               │  GATEWAY_BASE_URL=
                                ▼               ▼
                    ┌──────────────────┐   ┌──────────────────┐
                    │ relay-pubsub     │   │ Zyvor Relay      │
                    │ :8080/:8081      │──▶│ :8443 / :18080   │
                    └──────────────────┘   └────────┬─────────┘
                                                    │ Act (optional)
                                                    ▼
                                           ┌──────────────────┐
                                           │ farm / plant     │
                                           │ controllers      │
                                           └──────────────────┘
```

**Ports**

| Service | Typical port | Notes |
|---------|--------------|--------|
| relay-edge | 18086 | HTTPS default (`EDGE_TLS=1`) |
| relay-pubsub | 8080/8081 | Publish gateway |
| Relay | 8443 / 18080 | Accept / Notify / Act (site-specific listen port) |
| Forge (optional) | — | Not called by edge; Relay may gate Act |

Keep edge **off the public internet** unless Ingress + `EDGE_API_TOKEN` + real TLS are in place.

---

## Current maturity (2026-09-15)

| Claim | Status |
|---|---|
| Software matrix + CI (auth TLS, helm, kind-edge, backup) | green — image/chart **v0.1.2** |
| Lab host auth + TLS + backup | **signed** — [ops-checklist.md](https://github.com/zyvorai/relay-edge/blob/main/evidence/qualification/ops-checklist.md) |
| Defaults without token | still lab-open — set `EDGE_API_TOKEN` + `EDGE_REQUIRE_AUTH=1` |
| HA / multi-replica | **not supported** (`replicaCount: 1`) |

**Verdict:** relay-edge is **production-ready as a single-replica stamped feeder** when
items 1–4 below are applied (lab host has auth+TLS after redeploy).

## Production checklist

**Engineering preview defaults:** treat unset token as lab-open. Do not expose beyond a
trusted network until items 1–4 are done.

1. **API auth** — set `EDGE_API_TOKEN` (and `EDGE_REQUIRE_AUTH=1` so the process refuses to start without it). `./scripts/deploy-remote.sh` does this by default since it auto-generates a token when none is supplied. The Helm chart's own default is **not** auth-required (`edge.requireAuth: false`) — for k8s deploys, explicitly set `edge.requireAuth: true` and `edge.apiTokenKey` (pointing at a key in `edge.existingSecret`) the same way `values-production.yaml` should.
2. **TLS** — terminate at Ingress with a trusted cert, **or** mount a real cert via Helm `tls.existingSecret` (keys `cert.pem` / `key.pem`). Do not rely on auto-generated self-signed certs for users.
3. **Tokens** — use real Relay / gateway JWTs (`RELAY_AUTH_TOKEN`, `GATEWAY_AUTH_TOKEN`). Set `RELAY_TLS_INSECURE=0` once CAs trust Relay and pubsub. Left at its default (`1`, unset), relay-edge logs a startup warning that outbound TLS verification is disabled — this only fully closes once Relay/pubsub themselves present trusted certs, which is outside relay-edge's own scope to fix alone.
4. **Persistence** — PVC for `EDGE_DATA_DIR`; schedule [`scripts/backup-data.sh`](https://github.com/zyvorai/relay-edge/blob/main/scripts/backup-data.sh) — see [Backup and restore](#backup-and-restore) below for the systemd timer and Helm `backup.enabled` CronJob paths.
5. **Body limits** — default `EDGE_MAX_BODY_BYTES=8388608` (aligned with ingress `proxy-body-size: 8m`); lower if the site only posts small JSON events.
6. **Simulators / families** — set `EDGE_ENABLED_FAMILIES` to only what the site needs (`farm`, `firewater`, `remote-edge`, `fleet`), or leave empty for all. When the list is set, **farm** must be included explicitly or farm APIs stay off.
7. **Replicas** — keep `replicaCount: 1`. JSON file stores are not multi-writer safe.
8. **Metrics** — scrape `GET /metrics` (Prometheus text; `prometheus/client_golang`-backed, includes a request-duration histogram and Go runtime/process series alongside `relay_edge_*`). Path is public; protect via NetworkPolicy (`networkPolicy.enabled`, on in production values). A starting dashboard and alert rules are checked in at [`deploy/observability/grafana-dashboard.json`](https://github.com/zyvorai/relay-edge/blob/main/deploy/observability/grafana-dashboard.json) and [`deploy/observability/alerts.yaml`](https://github.com/zyvorai/relay-edge/blob/main/deploy/observability/alerts.yaml) — neither is wired into Helm/CI automatically, since that depends on how you run Prometheus/Grafana.
9. **Admin** — `/v1/admin/*` requires the same API token when auth is enabled. Enter the token in `/ui` → Configure → `edge_api_token` (browser localStorage). JWTs are not written to `runtime-config.json`.
10. **Qualify** — `make qualify` green; see [QUALIFICATION.md](QUALIFICATION.md). Graceful SIGTERM drain is 10s — set `terminationGracePeriodSeconds` ≥ 15.

**Lab E2E freshness**: [`.github/workflows/lab-e2e.yml`](https://github.com/zyvorai/relay-edge/blob/main/.github/workflows/lab-e2e.yml) scaffolds a weekly re-run of the full gateway/direct matrix against the real lab hosts, but it needs manual setup before it can pass: a Tailscale tailnet reaching the lab hosts, plus `TS_OAUTH_CLIENT_ID`/`TS_OAUTH_CLIENT_SECRET`/`LAB_212_RELAY_AUTH_TOKEN`/`LAB_175_RELAY_AUTH_TOKEN` repo secrets. Until that's wired up, [TEST_RESULTS.md](TEST_RESULTS.md)'s "Last re-run" date stays a manual process.

**When not to run relay-edge in production:** if you only need Relay Accept
from real devices/protocols — skip the simulators. Run this service when you
intentionally need stamped topology + synthetic or operator-driven events.

Helm starting point: [`deploy/helm/relay-edge/values-production.yaml`](https://github.com/zyvorai/relay-edge/blob/main/deploy/helm/relay-edge/values-production.yaml).

```bash
kubectl create secret generic relay-edge-secrets \
  --from-literal=relay-auth-token="$RELAY_JWT" \
  --from-literal=gateway-auth-token="$GW_JWT" \
  --from-literal=edge-api-token="$EDGE_API_TOKEN"

helm upgrade --install relay-edge ./deploy/helm/relay-edge \
  -f ./deploy/helm/relay-edge/values-production.yaml \
  --set image.tag=v0.1.2
```

---

## Auth

| Path | When `EDGE_API_TOKEN` set |
|------|---------------------------|
| `/healthz`, `/readyz`, `/version`, `/metrics` | Public |
| `/ui/*` | Public (static UI) |
| `/v1/*` including `/v1/admin/*` | `Authorization: Bearer <token>` or `X-Edge-Token: <token>` |

Unset `EDGE_API_TOKEN` = lab-open mode (warning logged). Scripts: `EDGE_API_TOKEN=… EDGE=https://… ./scripts/smoke.sh` (pass the header manually until scripts gain built-in support, or unset for lab).

---

## Backup and restore

```bash
EDGE_DATA_DIR=/var/lib/relay-edge/data ./scripts/backup-data.sh /backups/edge-$(date +%F).tgz
# stop edge, then:
EDGE_DATA_DIR=/var/lib/relay-edge/data ./scripts/restore-data.sh /backups/edge-….tgz
# start edge
```

Backup includes seasons/sites/zones/devices/contacts, `runtime-config.json`, and TLS files under the data dir if present.

**Scheduling it** — this is a manual script; nothing runs it automatically unless you wire one of these:

- **systemd targets** (`deploy-remote.sh`): `deploy-remote.sh` copies `scripts/backup-data.sh`/`restore-data.sh` to the remote deploy dir. Install [`deploy/systemd/relay-edge-backup.service`](https://github.com/zyvorai/relay-edge/blob/main/deploy/systemd/relay-edge-backup.service) + [`relay-edge-backup.timer`](https://github.com/zyvorai/relay-edge/blob/main/deploy/systemd/relay-edge-backup.timer) as a user unit (see the comment header in the `.service` file) for a daily backup into `~/.deployments/zyvor-relay-edge/backups/`.
- **Kubernetes / Helm**: set `backup.enabled: true` (plus optionally `backup.schedule`, `backup.size`) to render a `CronJob` (`deploy/helm/relay-edge/templates/backup-cronjob.yaml`) that runs `backup-data.sh` against the existing data PVC into a dedicated `<release>-backups` PVC, on `backup.schedule` (default daily at 03:00).

---

## Full stack install (high level)

1. Deploy **Relay** with production TLS and auth.
2. Deploy **relay-pubsub** pointed at Relay; sync gateway token.
3. Deploy **relay-edge** with gateway URL + tokens + `EDGE_API_TOKEN`.
4. `GET /readyz` → 200; `POST /v1/admin/probe` with Bearer token.
5. Optional: wire Relay Act → controllers (`lab-wire-relay-act.sh` is lab-oriented; adapt for production targets).
6. Run user smoke against the Ingress URL with `EDGE_API_TOKEN`.

See [INTEGRATION.md](INTEGRATION.md) and [RELAY.md](RELAY.md).

---

## Compatibility

| Component | Notes |
|-----------|--------|
| Go build | Go 1.22+ (see `go.mod`) |
| Kubernetes | 1.25+ recommended; Helm 3 |
| Relay | Zyvor Relay with `/v1/events` Accept API |
| relay-pubsub | Matching project topic publish path |
| Browsers | Modern Chromium/Safari/Firefox for `/ui` |
| Prometheus | Scrapes `/metrics` (OpenMetrics-ish text 0.0.4) |

Image: `ghcr.io/zyvorai/relay-edge:<tag>` — pin release tags, not `latest`.

---

## Support

| Channel | Use for |
|---------|---------|
| GitHub Issues | Bugs, feature requests (non-security) |
| security@zyvor.ai | Vulnerabilities — see [SECURITY.md](https://github.com/zyvorai/relay-edge/blob/main/SECURITY.md) |
| https://zyvor.dev | Product / vendor |

Include: edge version (`GET /version`), publish path (`gateway` vs `direct`), `auth_required` / `tls` from `/healthz`, and whether Ingress or raw `:18086` is used.

---

## Explicitly out of scope (today)

- Multi-replica HA writers on shared JSON stores
- Built-in ACME / Let’s Encrypt inside the binary
- IdP / OIDC (use Bearer edge token or put SSO at the Ingress)
- Full k8s e2e in CI without lab cluster secrets
