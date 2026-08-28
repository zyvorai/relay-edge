#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Probe relay-edge stack services before e2e runs.
#
# Usage:
#   source config/lab-stack.env   # optional
#   ./scripts/stack-probe.sh
#   ./scripts/stack-probe.sh --forge-optional
#   ./scripts/stack-probe.sh --direct          # edge + Relay only (no pubsub/Forge)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib/relay-api.sh
source "$SCRIPT_DIR/lib/relay-api.sh"

FORGE_OPTIONAL=0
DIRECT=0
for arg in "$@"; do
  case "$arg" in
    --forge-optional) FORGE_OPTIONAL=1 ;;
    --direct) DIRECT=1; FORGE_OPTIONAL=1 ;;
    -h|--help)
      echo "Usage: $0 [--forge-optional] [--direct]"
      exit 0
      ;;
  esac
done

relay_api_init
FAILED=0
pass() { echo "  ok  $1"; }
fail() { echo "  FAIL $1" >&2; FAILED=$((FAILED + 1)); }

if [[ "$DIRECT" -eq 1 ]]; then
  echo "== stack probe (direct) edge=$EDGE relay=$BASE =="
else
  echo "== stack probe edge=$EDGE gateway=$GATEWAY relay=$BASE forge=${FORGE_BASE:-<unset>} =="
fi

if curl -fsS "$EDGE/healthz" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("status")=="ok"' 2>/dev/null; then
  pass "relay-edge $EDGE/healthz"
else
  fail "relay-edge unreachable at $EDGE"
fi

if [[ "$DIRECT" -eq 1 ]]; then
  pass "relay-pubsub skipped (--direct)"
else
  if "${CURL_GW[@]}" "$GATEWAY/healthz" >/dev/null 2>&1; then
    pass "relay-pubsub $GATEWAY/healthz"
  else
    fail "relay-pubsub unreachable at $GATEWAY"
  fi
fi

if "${CURL_RELAY[@]}" "$BASE/healthz" >/dev/null 2>&1; then
  pass "Relay $BASE/healthz"
else
  fail "Relay unreachable at $BASE"
fi

if [[ -z "${FORGE_BASE:-}" || -z "${FORGE_API_KEY:-}" ]]; then
  if [[ "$FORGE_OPTIONAL" -eq 1 ]]; then
    pass "Forge skipped (--forge-optional, FORGE_BASE/FORGE_API_KEY unset)"
  else
    fail "Forge not configured (set FORGE_BASE + FORGE_API_KEY or use --forge-optional)"
  fi
else
  if relay_api_forge_probe >/dev/null 2>/tmp/forge-probe.err; then
    pass "Forge $FORGE_BASE/api/zeus/decisions"
  else
    if [[ "$FORGE_OPTIONAL" -eq 1 ]]; then
      pass "Forge unreachable but --forge-optional ($(head -c 80 /tmp/forge-probe.err))"
    else
      fail "Forge unreachable at $FORGE_BASE ($(head -c 120 /tmp/forge-probe.err))"
    fi
  fi
fi

echo ""
if [[ "$FAILED" -gt 0 ]]; then
  echo "stack-probe: $FAILED failure(s)" >&2
  exit 1
fi
echo "PASS: stack probe"
