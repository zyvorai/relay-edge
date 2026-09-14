---
hero:
  eyebrow: QUALIFICATION
  title: Qualification matrix — relay-edge
---

relay-edge is a **synthetic** topology/event companion for Zyvor Relay. There is
**no board hardware gate**. Qualification means: software matrix green, and a
conscious choice between lab/demo vs intentional stamped feeder.

## Software rows — `make qualify`

| ID | Expected |
|---|---|
| `helm_version_lockstep` | Chart `version` / `appVersion` = `0.1.1` |
| `gofmt` / `go_vet` / `unit_tests` / `unit_race` | Format, vet, unit suite, race detector |
| `govulncheck` | `govulncheck ./...` (or `go run …@latest`) |
| `build_binary` | `bin/relay-edge` |
| `docs_present` | PRODUCTION, QUALIFICATION, CHANGELOG |

## Lab vs production use

| Mode | When | Checklist |
|---|---|---|
| **Lab / demo** | Drive `/ui` simulators against mock or lab Relay | `EDGE_TLS` optional; auth optional; `--demo`-style tokens OK |
| **Intentional feeder** | Stamp real site topology into production Relay | [PRODUCTION.md](PRODUCTION.md) P0: auth-on, real TLS, `RELAY_TLS_INSECURE=0`, PVC, `replicaCount: 1` |

## Operator rows

| Test | Required outcome |
|---|---|
| Auth + HTTPS smoke | `EDGE_API_TOKEN` + `EDGE_REQUIRE_AUTH=1` + TLS; smokes pass |
| Publish path | Gateway or direct Accept succeeds against real Relay |
| Backup | `scripts/backup-data.sh` / restore |
| Family surface | `EDGE_ENABLED_FAMILIES` limited to needed families; include `farm` when farm APIs are required |
| Config hygiene | `runtime-config.json` has no JWT material after admin PUT |
| NetworkPolicy | Helm `networkPolicy.enabled=true` in production values |

## Lab host note

On `80.79.5.173`, `smoke-fleet.sh` passed; full `smoke.sh` returned **502**
without Zyvor Relay on `:8443` — expected until Relay is co-deployed.
