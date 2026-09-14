# Changelog

## Unreleased

- Fix qualify-matrix govulncheck when binary is absent; gofmt CI gate.
- GitHub CI: full auth-HTTPS smokes, backup/restore, helm kubeconform, kind
  edge smoke, CodeQL, suite-ci qualify markers.

## 0.1.2 — 2026-09-14

Production polish: body limits, race/govulncheck, graceful drain, fsync stores, farm gate, NetworkPolicy, JWT hygiene.

- Request body cap for POST/PUT/PATCH (`EDGE_MAX_BODY_BYTES`, default 8 MiB → 413).
- `go test -race` and `govulncheck` in CI / `make qualify`.
- Graceful SIGTERM/SIGINT shutdown with 10s drain.
- Atomic JSON writes fsync before rename (`jsonstore.WriteAtomic`).
- Runtime config no longer persists Relay/gateway JWTs (env/K8s secrets only).
- `EDGE_ENABLED_FAMILIES` can gate **farm** as well as simulator families.
- Helm optional `networkPolicy.enabled`.
- `EDGE_GCP_PROJECT` accepted alongside legacy `FASAL_GCP_PROJECT`.
- `make qualify` software matrix + QUALIFICATION docs (lab/demo vs intentional feeder).
- CI job: HTTPS + `EDGE_REQUIRE_AUTH` smoke path.
- Helm Chart/appVersion lockstep to **0.1.2**.
- Cosign keyless signature on release `SHA256SUMS`.
- README/FAQ maturity and `EDGE_TLS` default aligned with code (`true`).

## 0.1.1

- GitHub Release binaries (linux/darwin × amd64/arm64) and GHCR multi-arch image.
- Production runbook, Helm `values-production.yaml`, and lab TEST_RESULTS matrix.
- Auth-aware smoke scripts (`EDGE_API_TOKEN`).

## 0.1.0

- Initial public release: farm + firewater + remote-edge + fleet simulators,
  stamp-and-publish into Relay (gateway or direct), embedded `/ui` control rooms.
- systemd unit, container image, Helm chart, CI smokes vs mock Relay.

