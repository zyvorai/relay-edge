#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Deploy relay-edge to a remote host and point it at Relay + Pub/Sub gateway.
# Usage: ./scripts/deploy-remote.sh <HOST> [USER]
set -euo pipefail
if [[ $# -lt 1 || -z "${1:-}" ]]; then
  echo "usage: $0 <HOST> [USER]" >&2
  exit 1
fi
HOST="$1"
USER="${2:-sus}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# Isolated from other products that may share ~/.deployments/relay-edge
REMOTE_DIR="${REMOTE_DIR:-.deployments/zyvor-relay-edge}"
EDGE_PORT="${EDGE_PORT:-18086}"

echo "== relay-edge deploy → ${USER}@${HOST}:${REMOTE_DIR} :${EDGE_PORT} =="

cd "$ROOT"
mkdir -p bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/relay-edge-linux-amd64 ./cmd/relay-edge

ssh -o BatchMode=yes "${USER}@${HOST}" "mkdir -p ~/${REMOTE_DIR}/{bin,.run,data}"
scp -o BatchMode=yes bin/relay-edge-linux-amd64 "${USER}@${HOST}:/tmp/relay-edge.new"

TOK_FILE=$(mktemp)
if [[ -n "${RELAY_AUTH_TOKEN:-}" ]]; then
  printf '%s' "$RELAY_AUTH_TOKEN" >"$TOK_FILE"
elif [[ -f /tmp/lab-relay.jwt ]]; then
  cp /tmp/lab-relay.jwt "$TOK_FILE"
else
  : >"$TOK_FILE"
fi
scp -o BatchMode=yes "$TOK_FILE" "${USER}@${HOST}:/tmp/relay-edge.jwt"
rm -f "$TOK_FILE"

# Optional gateway shared secret
GW_FILE=$(mktemp)
if [[ -n "${GATEWAY_AUTH_TOKEN:-}" ]]; then
  printf '%s' "$GATEWAY_AUTH_TOKEN" >"$GW_FILE"
elif [[ -f /tmp/lab-gateway.token ]]; then
  cp /tmp/lab-gateway.token "$GW_FILE"
else
  : >"$GW_FILE"
fi
scp -o BatchMode=yes "$GW_FILE" "${USER}@${HOST}:/tmp/relay-edge-gateway.token"
rm -f "$GW_FILE"

ssh -o BatchMode=yes "${USER}@${HOST}" bash -s <<REMOTE
set -euo pipefail
# Free the port regardless of which deploy dir previously owned it
fuser -k ${EDGE_PORT}/tcp 2>/dev/null || true
pkill -f '/.deployments/zyvor-relay-edge/bin/relay-edge' || true
pkill -f '/.deployments/relay-edge/bin/relay-edge' || true
sleep 1
cd ~/${REMOTE_DIR}
mv -f /tmp/relay-edge.new ./bin/relay-edge
chmod +x ./bin/relay-edge
TOK=\$(cat /tmp/relay-edge.jwt 2>/dev/null || true)
GW=\$(cat /tmp/relay-edge-gateway.token 2>/dev/null || true)
export EDGE_HTTP_ADDR=:${EDGE_PORT}
export EDGE_DATA_DIR=\$HOME/${REMOTE_DIR}/data
export RELAY_BASE_URL=https://127.0.0.1:8443
export RELAY_TLS_INSECURE=1
if [[ "${RELAY_EDGE_DIRECT:-}" == "1" ]]; then
  export GATEWAY_BASE_URL=
else
  export GATEWAY_BASE_URL=https://127.0.0.1:8081
  export FASAL_GCP_PROJECT=fasal-onprem
fi
[[ -n "\$TOK" ]] && export RELAY_AUTH_TOKEN="\$TOK"
[[ -n "\$GW" ]] && export GATEWAY_AUTH_TOKEN="\$GW"
nohup ./bin/relay-edge > .run/edge.log 2>&1 &
echo \$! > .run/edge.pid
sleep 2
curl -fsS http://127.0.0.1:${EDGE_PORT}/healthz
echo
REMOTE

echo "OK: http://${HOST}:${EDGE_PORT}/healthz"
echo "Smoke: EDGE=http://${HOST}:${EDGE_PORT} ./scripts/smoke.sh"
