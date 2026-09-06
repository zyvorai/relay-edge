# Admin basics

| Variable | Purpose |
|----------|---------|
| `EDGE_HTTP_ADDR` | `:18086` |
| `EDGE_TLS` | `1` default HTTPS |
| `EDGE_API_TOKEN` | Optional Bearer for `/v1/*` |
| `GATEWAY_BASE_URL` | Pubsub host; empty string = direct Relay |
| `RELAY_BASE_URL` | Direct Relay host |
| `RELAY_AUTH_TOKEN` | Shared JWT |
| `RELAY_TLS_INSECURE` | Lab self-signed |
| `FASAL_GCP_PROJECT` | Gateway project segment |

Deploy: `./scripts/deploy-remote.sh <HOST> [USER]`. Production checklist: product-repo `docs/PRODUCTION.md`.

## Operate from the console (UX)

1. Open this route from the nav or command palette and wait for live API data.
2. Use filters/search when present; drill into a row for detail.
3. For mutating actions: confirm role gates and impact before applying.
4. **Empty / fail:** Check service health, auth, and that required CRDs/backends for this domain are installed.
5. **Success:** Live data loads; created/updated objects appear without error toasts.

