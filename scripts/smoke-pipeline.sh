#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0
# Smoke: deterministic seed, journal, replay dry run and chaos against a running edge.
# Usage: EDGE=http://127.0.0.1:18086 ./scripts/smoke-pipeline.sh
set -euo pipefail
EDGE="${EDGE:-https://127.0.0.1:18086}"
EDGE_API_TOKEN="${EDGE_API_TOKEN:-}"
curl_edge() {
  if [[ -n "$EDGE_API_TOKEN" ]]; then
    curl -fsSk -H "Authorization: Bearer ${EDGE_API_TOKEN}" "$@"
  else
    curl -fsSk "$@"
  fi
}

echo "== relay-edge pipeline smoke @ $EDGE =="
tick() { curl_edge -X POST "$EDGE/v1/fleet/tick" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)["snapshot"]["values"], sort_keys=True))'; }
curl_edge -X POST "$EDGE/v1/sim/seed" -H 'content-type: application/json' -d '{"seed":42}' >/dev/null
a="$(tick)"
curl_edge -X POST "$EDGE/v1/sim/seed" -H 'content-type: application/json' -d '{"seed":42}' >/dev/null
b="$(tick)"
[[ "$a" == "$b" ]] || { echo "FAIL: same seed gave different fleet values"; exit 1; }
curl_edge -X POST "$EDGE/v1/sim/seed" -H 'content-type: application/json' -d '{"seed":null}' >/dev/null
echo "seed determinism ok"

curl_edge -X POST "$EDGE/v1/replay" -H 'content-type: application/json' \
  -d '{"events":[{"type":"fleet.power.island","severity":"critical","data":{"n":1}}],"dry_run":true}' |
  python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]["dry_run"] and len(d["plan"])==1, d; print("replay dry run ok")'

code="$(curl -sk -o /dev/null -w '%{http_code}' ${EDGE_API_TOKEN:+-H "Authorization: Bearer ${EDGE_API_TOKEN}"} -X POST "$EDGE/v1/replay" -H 'content-type: application/json' -d '{"events":[{"type":"../x"}]}')"
[[ "$code" == "400" ]] || { echo "FAIL: hostile type got $code"; exit 1; }

curl_edge -X POST "$EDGE/v1/chaos/config" -H 'content-type: application/json' -d '{"enabled":true,"seed":1,"drop_pct":10}' >/dev/null
curl_edge "$EDGE/v1/chaos" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["config"]["enabled"]; print("chaos config ok")'
curl_edge -X POST "$EDGE/v1/chaos/reset" >/dev/null
curl_edge "$EDGE/v1/journal?limit=1" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "items" in d; print("journal ok, capacity", d["capacity"])'
curl_edge "$EDGE/v1/journal/export?format=cloudevents" >/dev/null

echo "PASS: pipeline smoke"
