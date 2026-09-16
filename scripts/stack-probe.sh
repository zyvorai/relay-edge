#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
# Probe relay-edge stack services before e2e runs.
#
# Usage:
#   source config/lab-stack.env   # optional
#   ./scripts/stack-probe.sh
#   ./scripts/stack-probe.sh --zynera-optional
#   ./scripts/stack-probe.sh --direct          # edge + Relay only (no pubsub/Zynera)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib/relay-api.sh
source "$SCRIPT_DIR/lib/relay-api.sh"

ZYNERA_OPTIONAL=0
DIRECT=0
for arg in "$@"; do
  case "$arg" in
    --zynera-optional) ZYNERA_OPTIONAL=1 ;;
    --direct) DIRECT=1; ZYNERA_OPTIONAL=1 ;;
    -h|--help)
      echo "Usage: $0 [--zynera-optional] [--direct]"
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
  echo "== stack probe edge=$EDGE gateway=$GATEWAY relay=$BASE zynera=${ZYNERA_BASE:-<unset>} =="
fi

if curl -fsSk "$EDGE/healthz" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("status")=="ok"' 2>/dev/null; then
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

if [[ -z "${ZYNERA_BASE:-}" || -z "${ZYNERA_API_KEY:-}" ]]; then
  if [[ "$ZYNERA_OPTIONAL" -eq 1 ]]; then
    pass "Zynera skipped (--zynera-optional, ZYNERA_BASE/ZYNERA_API_KEY unset)"
  else
    fail "Zynera not configured (set ZYNERA_BASE + ZYNERA_API_KEY or use --zynera-optional)"
  fi
else
  if relay_api_zynera_probe >/dev/null 2>/tmp/zynera-probe.err; then
    pass "Zynera $ZYNERA_BASE/api/zeus/decisions"
  else
    if [[ "$ZYNERA_OPTIONAL" -eq 1 ]]; then
      pass "Zynera unreachable but --zynera-optional ($(head -c 80 /tmp/zynera-probe.err))"
    else
      fail "Zynera unreachable at $ZYNERA_BASE ($(head -c 120 /tmp/zynera-probe.err))"
    fi
  fi
fi

echo ""
if [[ "$FAILED" -gt 0 ]]; then
  echo "stack-probe: $FAILED failure(s)" >&2
  exit 1
fi
echo "PASS: stack probe"
