#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Firewater smoke against relay-edge (HTTP or self-signed HTTPS).
# Usage: EDGE=https://127.0.0.1:28086 ./scripts/smoke-firewater-k8s.sh
set -euo pipefail
EDGE="${EDGE:-https://127.0.0.1:18086}"
CURL=(curl -fsS)
if [[ "$EDGE" == https:* ]]; then
  CURL=(curl -k -fsS)
fi

echo "== relay-edge firewater smoke @ $EDGE =="
"${CURL[@]}" "$EDGE/healthz" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert "firewater" in d.get("modules",[])
print("health ok:", ",".join(d.get("modules",[])))
'

"${CURL[@]}" -X POST "$EDGE/v1/firewater/seed" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["season"]["id"]=="season_fw_watch"
print("seed ok")
'

"${CURL[@]}" -X POST "$EDGE/v1/firewater/config" -H 'content-type: application/json' \
  -d '{"publish":false,"interval_ms":2000}' >/dev/null

"${CURL[@]}" -X POST "$EDGE/v1/firewater/scenario" -H 'content-type: application/json' \
  -d '{"scenario":"lowtank"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["values"]["tank_level"] < 40
print("lowtank ok")
'

echo "PASS: firewater smoke"
