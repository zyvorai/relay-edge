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
