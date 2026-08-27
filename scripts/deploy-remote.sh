#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Deploy relay-edge to a remote host and point it at Relay + Pub/Sub gateway.
# Usage: ./scripts/deploy-remote.sh [HOST] [USER]
set -euo pipefail
HOST="${1:-212.8.248.187}"
USER="${2:-sus}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REMOTE_DIR="${REMOTE_DIR:-.deployments/relay-edge}"
EDGE_PORT="${EDGE_PORT:-18086}"

echo "== relay-edge deploy → ${USER}@${HOST}:${REMOTE_DIR} :${EDGE_PORT} =="

cd "$ROOT"
mkdir -p bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/relay-edge-linux-amd64 ./cmd/relay-edge

ssh -o BatchMode=yes "${USER}@${HOST}" "mkdir -p ~/${REMOTE_DIR}/{bin,.run,data}"
scp -o BatchMode=yes bin/relay-edge-linux-amd64 "${USER}@${HOST}:~/${REMOTE_DIR}/bin/relay-edge"

# Fresh Relay JWT for direct fallback (gateway path needs gateway→Relay auth separately)
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

ssh -o BatchMode=yes "${USER}@${HOST}" bash -s <<REMOTE
set -euo pipefail
cd ~/${REMOTE_DIR}
PID=\$(pgrep -f '/.deployments/relay-edge/bin/relay-edge' | head -1 || true)
[[ -n "\${PID}" ]] && kill "\${PID}" || true
sleep 1
TOK=\$(cat /tmp/relay-edge.jwt 2>/dev/null || true)
export EDGE_HTTP_ADDR=:${EDGE_PORT}
export EDGE_DATA_DIR=~/${REMOTE_DIR}/data
export RELAY_BASE_URL=https://127.0.0.1:18080
export RELAY_TLS_INSECURE=1
export GATEWAY_BASE_URL=http://127.0.0.1:18083
export FASAL_GCP_PROJECT=fasal-onprem
[[ -n "\$TOK" ]] && export RELAY_AUTH_TOKEN="\$TOK"
nohup ./bin/relay-edge > .run/edge.log 2>&1 &
echo \$! > .run/edge.pid
sleep 2
curl -fsS http://127.0.0.1:${EDGE_PORT}/healthz
echo
REMOTE

echo "OK: http://${HOST}:${EDGE_PORT}/healthz"
echo "Smoke: EDGE=http://${HOST}:${EDGE_PORT} ./scripts/smoke.sh"
