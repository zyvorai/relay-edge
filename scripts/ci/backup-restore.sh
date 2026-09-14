#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
# CI: backup/restore EDGE_DATA_DIR round-trip.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
TMP=$(mktemp -d)
trap 'kill ${PID:-0} ${MOCK:-0} 2>/dev/null || true; rm -rf "$TMP"' EXIT
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=ci" -o bin/relay-edge ./cmd/relay-edge
python3 - <<'PY' &
from http.server import BaseHTTPRequestHandler, HTTPServer
class H(BaseHTTPRequestHandler):
    def _ok(self, body=b'{"status":"ok"}'):
        self.send_response(200); self.send_header("Content-Type","application/json"); self.end_headers(); self.wfile.write(body)
    def do_GET(self): self._ok()
    def do_POST(self):
        _=self.rfile.read(int(self.headers.get("Content-Length") or 0)); self._ok(b'{"event":{"id":"evt_ci"}}')
    def log_message(self,*a): pass
HTTPServer(("127.0.0.1",18080), H).serve_forever()
PY
MOCK=$!
export GATEWAY_BASE_URL=
export RELAY_BASE_URL=http://127.0.0.1:18080
export RELAY_TLS_INSECURE=1
export EDGE_TLS=0
export EDGE_HTTP_ADDR=:18086
export EDGE_DATA_DIR="$TMP/data"
mkdir -p "$EDGE_DATA_DIR"
./bin/relay-edge >"$TMP/edge.log" 2>&1 &
PID=$!
for i in $(seq 1 40); do curl -fsS http://127.0.0.1:18086/healthz >/dev/null 2>&1 && break; sleep 0.25; done
curl -fsS http://127.0.0.1:18086/healthz >/dev/null
# Touch admin config so store files exist
curl -fsS http://127.0.0.1:18086/v1/admin/config >/dev/null
kill "$PID"; wait "$PID" 2>/dev/null || true; PID=0
./scripts/backup-data.sh "$TMP/backup.tgz"
RESTORE="$TMP/restore-data"
mkdir -p "$RESTORE"
# restore-data extracts into EDGE_DATA_DIR; use empty target
rm -rf "$EDGE_DATA_DIR"
mkdir -p "$EDGE_DATA_DIR"
EDGE_DATA_DIR="$EDGE_DATA_DIR" ./scripts/restore-data.sh "$TMP/backup.tgz"
# rsync needs to be available — install hint if missing
command -v rsync >/dev/null || { echo "rsync required" >&2; exit 1; }
./bin/relay-edge >"$TMP/edge2.log" 2>&1 &
PID=$!
for i in $(seq 1 40); do curl -fsS http://127.0.0.1:18086/healthz >/dev/null 2>&1 && break; sleep 0.25; done
curl -fsS http://127.0.0.1:18086/healthz >/dev/null
EDGE=http://127.0.0.1:18086 ./scripts/smoke.sh
echo "PASS: relay-edge backup-restore"
