#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Direct Relay stack: health probe (edge + Relay) + expanded scenario matrix (no pubsub).
#
# Usage:
#   cp config/lab-direct.env.example config/lab-direct.env
#   # BASE, EDGE, RELAY_AUTH_TOKEN — relay-edge must have GATEWAY_BASE_URL= (direct mode)
#   set -a && source config/lab-direct.env && set +a
#   ./scripts/e2e-direct-stack.sh
#
# Deploy direct mode: RELAY_EDGE_DIRECT=1 RELAY_AUTH_TOKEN=... ./scripts/deploy-remote.sh <HOST>
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "== e2e direct stack — relay=${BASE:-https://127.0.0.1:8443} edge=${EDGE:-http://127.0.0.1:18086} (no gateway) =="

echo ""
echo "== Step 1: health probe (direct) =="
BASE="${BASE:-https://127.0.0.1:8443}" EDGE="${EDGE:-http://127.0.0.1:18086}" \
  bash "$SCRIPT_DIR/stack-probe.sh" --direct

echo ""
echo "== Step 2: direct Relay scenario matrix =="
BASE="${BASE:-https://127.0.0.1:8443}" EDGE="${EDGE:-http://127.0.0.1:18086}" \
  bash "$SCRIPT_DIR/e2e-direct-relay.sh"

echo ""
echo "PASS: e2e direct stack (relay-edge → Relay, no pubsub)"
