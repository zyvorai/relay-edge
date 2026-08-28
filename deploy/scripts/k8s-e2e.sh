#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Verify relay-pubsub + relay-edge pods (self-signed HTTPS) via port-forward.
#
# Run on the k8s host (or with KUBECONFIG pointing at the cluster).
# Usage:
#   NS_PUBSUB=relay-pubsub NS_EDGE=relay-edge ./deploy/scripts/k8s-e2e.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PUBSUB_ROOT="$(cd "${ROOT}/../relay-pubsub" && pwd)"
NS_PUBSUB="${NS_PUBSUB:-relay-pubsub}"
NS_EDGE="${NS_EDGE:-relay-edge}"
GW_PORT="${GW_PORT:-}"
EDGE_PORT="${EDGE_PORT:-}"
FAILED=0

pick_port() {
  local var=$1
  shift
  local candidates=("$@")
  for p in "${candidates[@]}"; do
    if ! ss -ltn 2>/dev/null | grep -q ":${p} "; then
      printf -v "$var" '%s' "$p"
      return 0
    fi
  done
  return 1
}

pick_port GW_PORT "${GW_PORT:-28081}" 38081 48081 58081
pick_port EDGE_PORT "${EDGE_PORT:-28086}" 38086 48086 58086

echo "== k8s e2e: gateway :${GW_PORT} edge :${EDGE_PORT} =="
kubectl -n "${NS_PUBSUB}" rollout status deployment/relay-pubsub --timeout=120s
kubectl -n "${NS_EDGE}" rollout status deployment/relay-edge --timeout=120s

PF_GW=""
PF_EDGE=""
cleanup() {
  [[ -n "$PF_GW" ]] && kill "$PF_GW" 2>/dev/null || true
  [[ -n "$PF_EDGE" ]] && kill "$PF_EDGE" 2>/dev/null || true
}
trap cleanup EXIT

kubectl -n "${NS_PUBSUB}" port-forward svc/relay-pubsub "${GW_PORT}:8080" >/tmp/pf-gw.log 2>&1 &
PF_GW=$!
kubectl -n "${NS_EDGE}" port-forward svc/relay-edge "${EDGE_PORT}:18086" >/tmp/pf-edge.log 2>&1 &
PF_EDGE=$!
sleep 2

for i in $(seq 1 30); do
  curl -ksf "https://127.0.0.1:${GW_PORT}/healthz" >/dev/null && \
  curl -ksf "https://127.0.0.1:${EDGE_PORT}/healthz" >/dev/null && break
  [[ "$i" -eq 30 ]] && { echo "health timeout"; cat /tmp/pf-gw.log /tmp/pf-edge.log; exit 1; }
  sleep 1
done
echo "  ✅ both pods healthy (self-signed HTTPS)"

echo "== relay-pubsub relay-events smoke =="
BASE="https://127.0.0.1:${GW_PORT}" bash "${PUBSUB_ROOT}/scripts/smoke-relay-events.sh"

echo "== relay-edge firewater smoke =="
EDGE="https://127.0.0.1:${EDGE_PORT}" bash "${ROOT}/scripts/smoke-firewater-k8s.sh"

echo "== publish path (edge → pubsub → relay) =="
curl -ksf -X POST "https://127.0.0.1:${EDGE_PORT}/v1/firewater/seed" >/dev/null
curl -ksf -X POST "https://127.0.0.1:${EDGE_PORT}/v1/firewater/config" \
  -H 'content-type: application/json' -d '{"publish":true,"interval_ms":5000}' >/dev/null
curl -ksf -X POST "https://127.0.0.1:${EDGE_PORT}/v1/firewater/scenario" \
  -H 'content-type: application/json' -d '{"scenario":"lowtank"}' >/dev/null
curl -ksf -X POST "https://127.0.0.1:${EDGE_PORT}/v1/remote-edge/config" \
  -H 'content-type: application/json' -d '{"publish":true}' >/dev/null
curl -ksf -X POST "https://127.0.0.1:${EDGE_PORT}/v1/remote-edge/scenario" \
  -H 'content-type: application/json' -d '{"scenario":"sat_down"}' >/dev/null
curl -ksf -X POST "https://127.0.0.1:${EDGE_PORT}/v1/fleet/config" \
  -H 'content-type: application/json' -d '{"publish":true}' >/dev/null
curl -ksf -X POST "https://127.0.0.1:${EDGE_PORT}/v1/fleet/scenario" \
  -H 'content-type: application/json' -d '{"scenario":"blackout"}' >/dev/null
echo "  ✅ edge simulators published (check Relay for firewater/remote-edge/fleet types)"

echo "PASS: k8s stack e2e"
