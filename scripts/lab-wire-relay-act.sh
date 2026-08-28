#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Wire lab Relay Act targets at relay-pubsub HTTPS /v1/actions + RELAY_TLS_INSECURE=1.
#
# Usage:
#   ./scripts/lab-wire-relay-act.sh <HOST> [USER]
#   RELAY_BIN=/path/to/linux-amd64-relay ./scripts/lab-wire-relay-act.sh <HOST>
#
# After wiring, re-sync pubsub JWT (restart invalidates tokens), then e2e-stack.
set -euo pipefail
HOST="${1:-}"
USER="${2:-sus}"
if [[ -z "$HOST" ]]; then
  echo "usage: $0 <HOST> [USER]" >&2
  exit 1
fi
REMOTE_DIR="${RELAY_REMOTE_DIR:-.deployments/zyvor-relay-enterprise}"
TARGETS='farm-controller=https://127.0.0.1:8081/v1/actions,firewater-controller=https://127.0.0.1:8081/v1/actions,remote-edge-controller=https://127.0.0.1:8081/v1/actions,fleet-controller=https://127.0.0.1:8081/v1/actions'

if [[ -n "${RELAY_BIN:-}" ]]; then
  echo "== scp Relay binary → ${USER}@${HOST} =="
  scp -o BatchMode=yes "$RELAY_BIN" "${USER}@${HOST}:/tmp/relay.new"
fi

ssh -o BatchMode=yes "${USER}@${HOST}" \
  env REMOTE_DIR="$REMOTE_DIR" TARGETS="$TARGETS" bash -s <<'REMOTE'
set -euo pipefail
DIR="$HOME/$REMOTE_DIR"
cd "$DIR"
mkdir -p .run bin
printf '%s\n' "RELAY_ACTION_TARGETS=$TARGETS" "RELAY_TLS_INSECURE=1" > .run/relay.env

python3 -c "
from pathlib import Path
import re
p = Path('$DIR') / '.env'
text = p.read_text() if p.exists() else ''
line = 'RELAY_ACTION_TARGETS=$TARGETS'
if 'RELAY_ACTION_TARGETS=' in text:
    text = re.sub(r'^RELAY_ACTION_TARGETS=.*$', line, text, flags=re.M)
else:
    text += '\n' + line + '\n'
if 'RELAY_TLS_INSECURE=' not in text:
    text += 'RELAY_TLS_INSECURE=1\n'
else:
    text = re.sub(r'^RELAY_TLS_INSECURE=.*$', 'RELAY_TLS_INSECURE=1', text, flags=re.M)
p.write_text(text)
print('updated', p)
"

pkill -f "/$REMOTE_DIR/bin/relay" || true
fuser -k 8443/tcp 2>/dev/null || true
sleep 2
if [[ -f /tmp/relay.new ]]; then
  mv -f /tmp/relay.new ./bin/relay
  chmod +x ./bin/relay
  echo "installed new Relay binary"
fi
set -a
# shellcheck disable=SC1091
[[ -f .env ]] && source .env
# shellcheck disable=SC1091
source .run/relay.env
set +a
export RELAY_ADDR=:8443 RELAY_PORT=8443 RELAY_TLS=true
export RELAY_TLS_CERT="$DIR/.run/tls/cert.pem" RELAY_TLS_KEY="$DIR/.run/tls/key.pem"
export RELAY_PUBLIC_BASE_URL=https://127.0.0.1:8443
export RELAY_TLS_INSECURE=1 RELAY_DEMO=true
export RELAY_DEMO_USER="${RELAY_DEMO_USER:-demo}" RELAY_DEMO_PASSWORD="${RELAY_DEMO_PASSWORD:-demo}"
export RELAY_TENANT="${RELAY_TENANT:-fasal-edge}" RELAY_STORAGE="${RELAY_STORAGE:-memory}"
export RELAY_JWT_SECRET="${RELAY_JWT_SECRET:-remote-demo-change-me}"
nohup ./bin/relay > .run/relay.log 2>&1 &
echo $! > .run/relay.pid
sleep 2
curl -fsSk https://127.0.0.1:8443/healthz
echo
tr '\0' '\n' < /proc/$(cat .run/relay.pid)/environ | grep -E 'ACTION_TARGETS|TLS_INSECURE' || true
REMOTE

echo ""
echo "OK: Relay Act wired to pubsub :8081"
echo "Next:"
echo "  TOKEN=\$(curl -fsSk -X POST https://${HOST}:8443/v1/auth/login -H 'content-type: application/json' -d '{\"username\":\"demo\",\"password\":\"demo\"}' | python3 -c 'import json,sys;print(json.load(sys.stdin)[\"token\"])')"
echo "  ssh ${USER}@${HOST} \"sudo sed -i \\\"s|^RELAY_AUTH_TOKEN=.*|RELAY_AUTH_TOKEN=\$TOKEN|\\\" /etc/relay-pubsub/relay-pubsub.env && sudo systemctl restart relay-pubsub\""
echo "  RELAY_AUTH_TOKEN=\$TOKEN ./scripts/deploy-remote.sh ${HOST} ${USER}"
echo "  BASE=https://${HOST}:8443 GATEWAY=https://${HOST}:8081 EDGE=http://${HOST}:18086 ./scripts/e2e-stack.sh"
