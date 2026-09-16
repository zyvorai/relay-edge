#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
# CI: fail if total statement coverage regresses below a threshold.
# Usage: ./scripts/ci/check-coverage.sh [threshold]
#   threshold defaults to 55 (percent). Measured baseline at the time this
#   gate was added was 62.4% -- the default leaves headroom so unrelated
#   PRs aren't blocked by minor coverage noise, while still catching a
#   real regression.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
THRESHOLD="${1:-55}"

go test -coverprofile=coverage.out ./... >/dev/null
TOTAL_PCT=$(go tool cover -func=coverage.out | tail -1 | awk '{print $NF}' | tr -d '%')

echo "total coverage: ${TOTAL_PCT}% (threshold: ${THRESHOLD}%)"

awk -v total="$TOTAL_PCT" -v threshold="$THRESHOLD" 'BEGIN { exit !(total >= threshold) }' \
  || { echo "FAIL: coverage ${TOTAL_PCT}% is below the ${THRESHOLD}% threshold" >&2; exit 1; }

echo "PASS: coverage ${TOTAL_PCT}% >= ${THRESHOLD}%"
