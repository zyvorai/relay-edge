#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Deploy relay-edge to a remote host and point it at Relay + Pub/Sub gateway.
# Prefers systemd when available; falls back to nohup.
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
USE_SYSTEMD="${USE_SYSTEMD:-auto}"

echo "== relay-edge deploy → ${USER}@${HOST}:${REMOTE_DIR} :${EDGE_PORT} =="

cd "$ROOT"
mkdir -p bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/relay-edge-linux-amd64 ./cmd/relay-edge

ssh -o BatchMode=yes "${USER}@${HOST}" "mkdir -p ~/${REMOTE_DIR}/{bin,.run,data}"
scp -o BatchMode=yes bin/relay-edge-linux-amd64 "${USER}@${HOST}:/tmp/relay-edge.new"
scp -o BatchMode=yes "$ROOT/deploy/systemd/relay-edge.service" "${USER}@${HOST}:/tmp/relay-edge.service"

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
USE_SYSTEMD='${USE_SYSTEMD}'
# Free the port regardless of which deploy dir previously owned it
fuser -k ${EDGE_PORT}/tcp 2>/dev/null || true
pkill -f '/.deployments/zyvor-relay-edge/bin/relay-edge' || true
pkill -f '/.deployments/relay-edge/bin/relay-edge' || true
sudo systemctl stop relay-edge 2>/dev/null || true
sleep 1
cd ~/${REMOTE_DIR}
mv -f /tmp/relay-edge.new ./bin/relay-edge
chmod +x ./bin/relay-edge
TOK=\$(cat /tmp/relay-edge.jwt 2>/dev/null || true)
GW=\$(cat /tmp/relay-edge-gateway.token 2>/dev/null || true)

ENV_FILE=\$HOME/${REMOTE_DIR}/relay-edge.env
{
  echo "EDGE_HTTP_ADDR=:${EDGE_PORT}"
  echo "EDGE_DATA_DIR=\$HOME/${REMOTE_DIR}/data"
  echo "EDGE_TLS=1"
  echo "EDGE_TLS_CERT=\$HOME/${REMOTE_DIR}/data/tls/cert.pem"
  echo "EDGE_TLS_KEY=\$HOME/${REMOTE_DIR}/data/tls/key.pem"
  echo "EDGE_TLS_SAN=localhost,127.0.0.1,${HOST},relay-edge"
  echo "RELAY_BASE_URL=https://127.0.0.1:8443"
  echo "RELAY_TLS_INSECURE=1"
  if [[ "${RELAY_EDGE_DIRECT:-}" == "1" ]]; then
    echo "GATEWAY_BASE_URL="
  else
    echo "GATEWAY_BASE_URL=https://127.0.0.1:8081"
    echo "FASAL_GCP_PROJECT=fasal-onprem"
  fi
  [[ -n "\$TOK" ]] && echo "RELAY_AUTH_TOKEN=\$TOK"
  [[ -n "\$GW" ]] && echo "GATEWAY_AUTH_TOKEN=\$GW"
} > "\$ENV_FILE"

want_systemd=0
case "\$USE_SYSTEMD" in
  1|true|yes|on) want_systemd=1 ;;
  0|false|no|off) want_systemd=0 ;;
  *)
    if command -v systemctl >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
      want_systemd=1
    fi
    ;;
esac

if [[ "\$want_systemd" -eq 1 ]]; then
  UNIT=/tmp/relay-edge.service
  sed -e "s|User=relay-edge|User=\$(whoami)|" \
      -e "s|Group=relay-edge|Group=\$(id -gn)|" \
      -e "s|WorkingDirectory=/var/lib/relay-edge|WorkingDirectory=\$HOME/${REMOTE_DIR}|" \
      -e "s|EnvironmentFile=-/etc/relay-edge/relay-edge.env|EnvironmentFile=\$ENV_FILE|" \
      -e "s|ExecStart=/usr/local/bin/relay-edge|ExecStart=\$HOME/${REMOTE_DIR}/bin/relay-edge|" \
      -e "s|ReadWritePaths=/var/lib/relay-edge|ReadWritePaths=\$HOME/${REMOTE_DIR}|" \
      -e "s|ProtectHome=true|ProtectHome=false|" \
      "\$UNIT" | sudo tee /etc/systemd/system/relay-edge.service >/dev/null
  sudo systemctl daemon-reload
  sudo systemctl enable --now relay-edge
  echo "started via systemd"
else
  set -a
  # shellcheck disable=SC1090
  source "\$ENV_FILE"
  set +a
  nohup ./bin/relay-edge > .run/edge.log 2>&1 &
  echo \$! > .run/edge.pid
  echo "started via nohup (pid \$(cat .run/edge.pid))"
fi
sleep 2
curl -fsSk http://127.0.0.1:${EDGE_PORT}/healthz 2>/dev/null || curl -fsSk https://127.0.0.1:${EDGE_PORT}/healthz
echo
curl -fsSk http://127.0.0.1:${EDGE_PORT}/readyz 2>/dev/null || curl -fsSk https://127.0.0.1:${EDGE_PORT}/readyz
echo
REMOTE

echo "OK: https://${HOST}:${EDGE_PORT}/ui/  (self-signed — accept browser warning)"
echo "    http may still work only if EDGE_TLS=0"
echo "Smoke: EDGE=https://${HOST}:${EDGE_PORT} ./scripts/smoke.sh"
echo "       EDGE=https://${HOST}:${EDGE_PORT} ./scripts/smoke-fleet.sh"
