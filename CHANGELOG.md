# Changelog

## Unreleased

**Security**

- **Go toolchain 1.23 → 1.27** everywhere (`go.mod`, Dockerfile builder
  image, every CI workflow). Go 1.23 was EOL with unpatched CVEs in
  `crypto/tls`, `net/http`, `net/url`, `encoding/asn1`, `net/textproto`,
  all reachable through relay-edge's own TLS/HTTP/publish code;
  `govulncheck` now reports zero vulnerabilities (was 5). Dependencies
  bumped to current (`prometheus/client_golang` 1.19.1 → 1.24.1 and
  transitives).
- Fixed a data race in `internal/fleet` and `internal/remoteedge`:
  `Engine.snap()` returned `Snapshot.Values` as a live reference into the
  engine's internal map instead of a clone, so a concurrent tick
  mutating that map could race with an HTTP handler JSON-marshaling the
  snapshot. `internal/firewater` already had the correct pattern.
- Fixed all three SSE stream endpoints (`/v1/firewater/stream`,
  `/v1/fleet/stream`, `/v1/remote-edge/stream`), which unconditionally
  500'd (`"stream unsupported"`) for any real client: `withMetrics`'s
  `statusRecorder` wrapper embedded `http.ResponseWriter` by its
  interface type, so `Flush` was never promoted and the handlers'
  `w.(http.Flusher)` check always failed.
- `deploy-remote.sh` now defaults to auth-on: auto-generates
  `EDGE_API_TOKEN` and sets `EDGE_REQUIRE_AUTH=1` unless explicitly
  overridden (with a loud warning + `CONFIRM_INSECURE=1` gate if you
  do), cleans up the plaintext secret files it used to leave in the
  remote host's `/tmp`, and `chmod 600`s the generated env file.
- Per-client-IP rate limiting (`EDGE_RATE_LIMIT_RPS` /
  `EDGE_RATE_LIMIT_BURST`, hand-rolled token bucket, off by default) sits
  ahead of auth so repeated bad-token guesses get throttled too.
- Startup warning when `RELAY_TLS_INSECURE` is left at its default
  (`true`, unchanged) instead of being silently insecure.

**Observability**

- Adopted `prometheus/client_golang` (first third-party dependency) for
  `/metrics` — preserves the existing `relay_edge_*` series names/values,
  adds a `relay_edge_http_request_duration_seconds` histogram (labeled
  by bounded route pattern, not raw path) plus Go runtime/process
  collectors.
- Migrated logging to `log/slog`; format selectable via
  `EDGE_LOG_FORMAT=text|json` (default `text`, matches the existing
  `/ui` Logs tab / `/v1/admin/logs` output).
- New `deploy/observability/grafana-dashboard.json` and `alerts.yaml`
  (uptime, request/error/publish rate, p50/p95 duration, Go runtime
  memory/CPU) — static, reference-only, not auto-wired into Helm/CI.

**CI / quality**

- `.golangci.yml` (5 linters) wired into `ci.yml` and `release-image.yml`
  (the latter previously pushed GHCR images with no test/vet/lint gate
  of its own).
- Coverage gate (`scripts/ci/check-coverage.sh`, threshold 55%) in CI;
  overall repo coverage rose from ~33% (`internal/httpapi`) to a blended
  ~65% total across many new tests, including the two bugs above.
- `scripts/qualify-matrix.py`'s version check now compares `Makefile`,
  the script itself, and Helm `Chart.yaml` three ways (previously only
  checked `Chart.yaml` against itself).
- New `scripts/rollback-remote.sh` (restores the previous binary a
  `deploy-remote.sh` run kept); `deploy-k8s-remote.sh` now tags images by
  commit instead of mutable `:latest` so `helm rollback` works.
- `.github/dependabot.yml` gained a `gomod` ecosystem entry.
- New `.github/workflows/lab-e2e.yml` scaffolds a weekly lab E2E re-run
  against real hosts + auto-PRs a `TEST_RESULTS.md` date bump on
  success — needs Tailscale/secret setup before it can actually run.
- Scheduled backups: systemd timer (`deploy/systemd/relay-edge-backup.*`)
  and an opt-in Helm `backup.enabled` CronJob, both reusing the existing
  `scripts/backup-data.sh`.

**Docs / branding**

- README banner + a real screenshot gallery of the `/ui` control rooms
  (previously ASCII-art only); docs site now emits `og:image`/
  `twitter:image` meta tags from the existing (previously unused) social
  preview image.
- Renamed "Forge" → "Zynera" throughout (branding only — `docs/FORGE.md`
  → `docs/ZYNERA.md`, relay-edge's own script vars `FORGE_BASE`/
  `FORGE_API_KEY` → `ZYNERA_BASE`/`ZYNERA_API_KEY`, `e2e-forge-stack.sh`
  → `e2e-zynera-stack.sh`). Relay's own contract identifiers
  (`RELAY_FORGE_BASE_URL`, `decision_backend: forge`,
  `forge_decision_record_id`) are untouched since Relay's actual source
  still uses them.

- Docs refresh: pin Helm/QUALIFICATION/README/FAQ to **v0.1.2**; document
  lab `EDGE_REQUIRE_AUTH` maturity vs lab-open defaults.

- Redeploy lab with EDGE_REQUIRE_AUTH; signed ops checklist; deploy-remote writes REQUIRE_AUTH.

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

