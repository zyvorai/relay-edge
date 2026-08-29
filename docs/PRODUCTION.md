# Production runbook

Customer / site deployment of **relay-edge** (edge companion for Zyvor Relay).

← [Docs hub](README.md) · [Deployment](DEPLOYMENT.md) · [Security](../SECURITY.md)

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

## Production checklist

1. **API auth** — set `EDGE_API_TOKEN` (and `EDGE_REQUIRE_AUTH=1` so the process refuses to start without it).
2. **TLS** — terminate at Ingress with a trusted cert, **or** mount a real cert via Helm `tls.existingSecret` (keys `cert.pem` / `key.pem`). Do not rely on auto-generated self-signed certs for customers.
3. **Tokens** — use real Relay / gateway JWTs (`RELAY_AUTH_TOKEN`, `GATEWAY_AUTH_TOKEN`). Set `RELAY_TLS_INSECURE=0` once CAs trust Relay and pubsub.
4. **Persistence** — PVC for `EDGE_DATA_DIR`; schedule [`scripts/backup-data.sh`](../scripts/backup-data.sh).
5. **Simulators** — set `EDGE_ENABLED_FAMILIES` to only what the site needs, or leave empty for all. Lab plant UIs are not required in production.
6. **Replicas** — keep `replicaCount: 1`. JSON file stores are not multi-writer safe.
7. **Metrics** — scrape `GET /metrics` (Prometheus text). Path is public; protect via network policy if needed.
8. **Admin** — `/v1/admin/*` requires the same API token when auth is enabled. Enter the token in `/ui` → Configure → `edge_api_token` (browser localStorage).

Helm starting point: [`deploy/helm/relay-edge/values-production.yaml`](../deploy/helm/relay-edge/values-production.yaml).

```bash
kubectl create secret generic relay-edge-secrets \
  --from-literal=relay-auth-token="$RELAY_JWT" \
  --from-literal=gateway-auth-token="$GW_JWT" \
  --from-literal=edge-api-token="$EDGE_API_TOKEN"

helm upgrade --install relay-edge ./deploy/helm/relay-edge \
  -f ./deploy/helm/relay-edge/values-production.yaml \
  --set image.tag=v0.1.1
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

---

## Full stack install (high level)

1. Deploy **Relay** with production TLS and auth.
2. Deploy **relay-pubsub** pointed at Relay; sync gateway token.
3. Deploy **relay-edge** with gateway URL + tokens + `EDGE_API_TOKEN`.
4. `GET /readyz` → 200; `POST /v1/admin/probe` with Bearer token.
5. Optional: wire Relay Act → controllers (`lab-wire-relay-act.sh` is lab-oriented; adapt for production targets).
6. Run customer smoke against the Ingress URL with `EDGE_API_TOKEN`.

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
| security@zyvor.ai | Vulnerabilities — see [SECURITY.md](../SECURITY.md) |
| https://zyvor.dev | Product / vendor |

Include: edge version (`GET /version`), publish path (`gateway` vs `direct`), `auth_required` / `tls` from `/healthz`, and whether Ingress or raw `:18086` is used.

---

## Explicitly out of scope (today)

- Multi-replica HA writers on shared JSON stores
- Built-in ACME / Let’s Encrypt inside the binary
- IdP / OIDC (use Bearer edge token or put SSO at the Ingress)
- Full k8s e2e in CI without lab cluster secrets
