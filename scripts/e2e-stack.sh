#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Stack verification when Forge is not deployed: health probe + full event matrix.
#
# Usage:
#   cp config/lab-stack.env.example config/lab-stack.env
#   # Set BASE, GATEWAY, EDGE, RELAY_AUTH_TOKEN — leave FORGE_* empty
#   set -a && source config/lab-stack.env && set +a
#   ./scripts/e2e-stack.sh
#
# Does not require FORGE_BASE, FORGE_API_KEY, or RELAY_FORGE_* on Relay.
# When Forge is co-located, use ./scripts/e2e-forge-stack.sh instead.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "== e2e stack (Forge not required) — relay=${BASE:-https://127.0.0.1:8443} gateway=${GATEWAY:-https://127.0.0.1:8081} edge=${EDGE:-http://127.0.0.1:18086} =="

echo ""
echo "== Step 1: health probe =="
BASE="${BASE:-https://127.0.0.1:8443}" GATEWAY="${GATEWAY:-https://127.0.0.1:8081}" \
  EDGE="${EDGE:-http://127.0.0.1:18086}" \
  bash "$SCRIPT_DIR/stack-probe.sh" --forge-optional

echo ""
echo "== Step 2: event matrix =="
BASE="${BASE:-https://127.0.0.1:8443}" GATEWAY="${GATEWAY:-https://127.0.0.1:8081}" \
  EDGE="${EDGE:-http://127.0.0.1:18086}" \
  bash "$SCRIPT_DIR/e2e-events-matrix.sh"

echo ""
echo "PASS: e2e stack (relay-edge + pubsub + Relay — no Forge)"
