# relay-edge ops qualification checklist

**Status:** signed for lab host `80.79.5.173` after redeploy with auth+TLS.
Does **not** claim multi-replica HA, real Relay JWT trust without `RELAY_TLS_INSECURE`,
or multi-day soak.

| Field | Value |
|---|---|
| Operator | lab-ops-drill |
| Date (UTC) | 2026-09-14 |
| Binary | redeployed from `main` `@26c9839` (+ deploy-remote auth patch) |
| Host | `80.79.5.173` systemd `relay-edge.service` |
| Evidence | `evidence/qualification/lab/20260914T155128Z/` |

## Results

| Test | Result | Notes |
|---|---|---|
| API auth (`EDGE_API_TOKEN` + `EDGE_REQUIRE_AUTH=1`) | pass | `/healthz` shows `auth_required:true`; `/v1/admin/*` → 401 without Bearer |
| TLS | pass | HTTPS `:18086` (self-signed lab cert) |
| Persistence + backup | pass | `backup-data.sh` → archive sha256 `853bba2b…` |
| Single replica | pass | `replicaCount`/one systemd unit |
| Relay trust hardened (`RELAY_TLS_INSECURE=0`) | blocked | lab still `RELAY_TLS_INSECURE=1` against local Relay |
| NetworkPolicy / metrics lock-down | blocked | lab host evaluation |

## Sign-off

- Name: lab-ops-drill
- Token fingerprint (sha256 of `EDGE_API_TOKEN`): see `lab/20260914T155128Z/relay-edge-token.sha256`
- Production Helm starting point: `deploy/helm/relay-edge/values-production.yaml`
