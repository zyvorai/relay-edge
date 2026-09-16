#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
# Roll a remote relay-edge deploy (from deploy-remote.sh) back to the
# previous binary. Requires at least two prior deploy-remote.sh runs, since
# the "previous" binary is only saved starting on a deploy's second run.
# Usage: ./scripts/rollback-remote.sh <HOST> [USER]
set -euo pipefail
if [[ $# -lt 1 || -z "${1:-}" ]]; then
  echo "usage: $0 <HOST> [USER]" >&2
  exit 1
fi
HOST="$1"
USER="${2:-sus}"
REMOTE_DIR="${REMOTE_DIR:-.deployments/zyvor-relay-edge}"
EDGE_PORT="${EDGE_PORT:-18086}"
USE_SYSTEMD="${USE_SYSTEMD:-auto}"

echo "== relay-edge rollback → ${USER}@${HOST}:${REMOTE_DIR} :${EDGE_PORT} =="

ssh -o BatchMode=yes "${USER}@${HOST}" bash -s <<REMOTE
set -euo pipefail
USE_SYSTEMD='${USE_SYSTEMD}'
cd ~/${REMOTE_DIR}
if [[ ! -f ./bin/relay-edge.previous ]]; then
  echo "no previous binary at ~/${REMOTE_DIR}/bin/relay-edge.previous" >&2
  echo "run deploy-remote.sh at least twice, or restore from backup-data.sh" >&2
  exit 1
fi

fuser -k ${EDGE_PORT}/tcp 2>/dev/null || true
pkill -f '/.deployments/zyvor-relay-edge/bin/relay-edge' || true
sudo systemctl stop relay-edge 2>/dev/null || true
sleep 1

mv -f ./bin/relay-edge ./bin/relay-edge.rolled-back
mv -f ./bin/relay-edge.previous ./bin/relay-edge
chmod +x ./bin/relay-edge

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
  sudo systemctl start relay-edge
  echo "restarted via systemd"
else
  ENV_FILE=\$HOME/${REMOTE_DIR}/relay-edge.env
  set -a
  # shellcheck disable=SC1090
  source "\$ENV_FILE"
  set +a
  nohup ./bin/relay-edge > .run/edge.log 2>&1 &
  echo \$! > .run/edge.pid
  echo "restarted via nohup (pid \$(cat .run/edge.pid))"
fi
sleep 2
curl -fsSk http://127.0.0.1:${EDGE_PORT}/healthz 2>/dev/null || curl -fsSk https://127.0.0.1:${EDGE_PORT}/healthz
echo
curl -fsSk http://127.0.0.1:${EDGE_PORT}/readyz 2>/dev/null || curl -fsSk https://127.0.0.1:${EDGE_PORT}/readyz
echo
REMOTE

echo "OK: rolled back and restarted at https://${HOST}:${EDGE_PORT}/ui/"
