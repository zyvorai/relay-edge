# Changelog

## 0.1.1

- GitHub Release binaries (linux/darwin × amd64/arm64) and GHCR multi-arch image.
- Production runbook, Helm `values-production.yaml`, and lab TEST_RESULTS matrix.
- Auth-aware smoke scripts (`EDGE_API_TOKEN`).

## 0.1.0

- Initial public release: farm + firewater + remote-edge + fleet simulators,
  stamp-and-publish into Relay (gateway or direct), embedded `/ui` control rooms.
- systemd unit, container image, Helm chart, CI smokes vs mock Relay.

## Unreleased

- `make qualify` software matrix + QUALIFICATION docs (lab/demo vs intentional feeder).
- CI job: HTTPS + `EDGE_REQUIRE_AUTH` smoke path.
- Helm Chart/appVersion lockstep to **0.1.1**.
- Cosign keyless signature on release `SHA256SUMS`.
- README/FAQ maturity and `EDGE_TLS` default aligned with code (`true`).
