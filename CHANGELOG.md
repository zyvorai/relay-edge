# Changelog

## Unreleased

- **Deterministic simulation.** `EDGE_SIM_SEED` / `POST /v1/sim/seed` give the
  fleet, remote-edge and firewater engines per-engine seeded noise and
  counter-based idempotency keys, so the same seed and tick sequence yields the
  same values and keys. Unseeded behavior is unchanged.
- **Publish journal.** Bounded in-memory ring (`EDGE_JOURNAL_MAX`, default 5000)
  of every stamped simulator event with origin and outcome;
  `GET /v1/journal`, `GET /v1/journal/export` as NDJSON or CloudEvents 1.0
  batch, `POST /v1/journal/clear`.
- **Replay.** `POST /v1/replay` re-publishes the journal or an uploaded
  NDJSON / CloudEvents / JSON stream at `speed` 0.01 to 1000 with capped
  pauses, at most 10000 events per run, one run at a time, dry run, status and
  cancel. Keys are `replay/<run_id>/<n>`, so a repeat run is idempotent at Relay.
- **Chaos on the publish path.** `POST /v1/chaos/config` injects seeded drop,
  duplicate, error and delay/jitter faults (bounded, unknown fields rejected);
  counters in `GET /v1/chaos` and `relay_edge_publish_decisions_total`.
- `/ui` Pipeline panel, `scripts/smoke-pipeline.sh` (in `make smoke-all` and
  CI), `docs/REPLAY.md`, `docs/RESEARCH.md`.
- `make help` and `make ci` (gofmt, vet, tests, build).

**Fixes**

- `relaypub` now rejects event types outside `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`:
  on the gateway path the type is a URL path segment, so a type such as
  `../admin` or `a?x=1` from `POST /v1/seasons/{id}/events` reached the URL
  unescaped.
- Remote-edge and firewater `healthyFleet` ranged over a Go map while drawing
  random numbers, so even with a fixed seed the values would differ run to
  run; keys are now visited in sorted order.

**Upgrades**

- Go modules: `go get -u ./... && go mod tidy` (`prometheus/common` 0.72 -> 0.74,
  `procfs` 0.22 -> 0.23, `x/sys` 0.48 -> 0.50; new indirect `pandatix/go-abnf`
  pulled in by `prometheus/common`). The only direct dependency,
  `prometheus/client_golang` v1.25.0, is already latest and has no newer major
  module path.
- Dockerfile: builder `golang:1.27-trixie`, runtime `debian:trixie-slim`
  (tags verified on Docker Hub). CI mock-Relay pod `python:3.12-alpine` ->
  `3.14-alpine`; docs workflow `python-version` `3.x` -> `3.14`.
- `sigstore/cosign-installer` v3 -> v4. `requirements-docs.txt` floors raised
  to `mkdocs>=1.6.1`, `mkdocs-material>=9.8` (PyPI latest 1.6.1 / 9.8.2).
- Already current and unchanged: `go 1.27` directive and CI go-version, every
  other pinned `actions/*`, `docker/*`, `azure/setup-helm@v5`, `github/codeql-action@v4`.

**Held back / unverified**

- CI Go matrix 1.26.x/1.27.x not adopted: `go.mod` requires `go 1.27`, so a
  1.26.x leg could not build without a toolchain download. Single 1.27 leg kept.
- Unverified latest majors, left as-is: `golangci/golangci-lint-action@v9`
  (+ `golangci-lint v2.13`), `helm/kind-action@v1`,
  `peter-evans/create-pull-request@v8`, `tailscale/github-action@v4`,
  `govulncheck@v1.8.0`, `kubeconform@latest`.
- No Node/npm tooling exists in this repo (`web/` is static embedded HTML), so
  there are no Node pins to move.

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

