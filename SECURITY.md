# Security Policy

## Reporting a vulnerability

Email **security@zyvor.ai** (or open a private GitHub security advisory on this repo). Please include reproduction steps and impact.

Do not file public issues for undisclosed vulnerabilities.

## Scope

relay-edge is an edge companion / simulator that stamps and publishes events. By default it exposes an **unauthenticated** HTTP API (lab-friendly). Harden before production:

- Put TLS and network policy / reverse-proxy auth in front of `:18086`
- Prefer gateway mode with short-lived `RELAY_AUTH_TOKEN` / `GATEWAY_AUTH_TOKEN`
- Set `RELAY_TLS_INSECURE=0` and trust real CAs outside the lab
- Restrict `EDGE_ENABLED_FAMILIES` if only a subset of simulators should run
