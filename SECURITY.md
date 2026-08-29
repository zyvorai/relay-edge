# Security Policy

## Reporting a vulnerability

Email **security@zyvor.ai** (or open a private GitHub security advisory on this repo). Please include reproduction steps and impact.

Do not file public issues for undisclosed vulnerabilities.

## Scope

relay-edge is an edge companion / simulator that stamps and publishes events. Without `EDGE_API_TOKEN` it exposes an **unauthenticated** HTTP API (lab-friendly). Harden before production:

- Set `EDGE_API_TOKEN` (and preferably `EDGE_REQUIRE_AUTH=1`) so `/v1/*` requires Bearer auth
- Put real TLS at Ingress or via `tls.existingSecret` — do not ship lab self-signed certs to customers
- Prefer gateway mode with short-lived `RELAY_AUTH_TOKEN` / `GATEWAY_AUTH_TOKEN`
- Set `RELAY_TLS_INSECURE=0` and trust real CAs outside the lab
- Restrict `EDGE_ENABLED_FAMILIES` if only a subset of simulators should run
- See [docs/PRODUCTION.md](docs/PRODUCTION.md) for the full customer checklist
