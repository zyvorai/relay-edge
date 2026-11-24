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

# Rate pacing, shuffle, journal stats, resend with overrides, diff (round 3).
EVS='[{"type":"smoke.a","severity":"info","data":{"i":0,"drop":1}},{"type":"smoke.a","severity":"critical","data":{"i":1}},{"type":"smoke.b","severity":"info","data":{"i":2}},{"type":"smoke.b","severity":"info","data":{"i":3}}]'
curl_edge -X POST "$EDGE/v1/replay" -H 'content-type: application/json' \
  -d "{\"events\":$EVS,\"dry_run\":true,\"rate_stages\":[{\"duration_ms\":1000,\"rate_per_sec\":20}],\"start_rate\":5,\"shuffle_window\":4,\"shuffle_seed\":1}" |
  python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]["pacing"]=="rate" and len(d["plan"])==4, d; print("replay rate+shuffle dry run ok")'
code="$(curl -sk -o /dev/null -w '%{http_code}' ${EDGE_API_TOKEN:+-H "Authorization: Bearer ${EDGE_API_TOKEN}"} -X POST "$EDGE/v1/replay" -H 'content-type: application/json' -d "{\"events\":$EVS,\"rate_per_sec\":5,\"speed\":2}")"
[[ "$code" == "400" ]] || { echo "FAIL: rate+speed got $code"; exit 1; }
curl_edge -X POST "$EDGE/v1/journal/clear" >/dev/null
curl_edge -X POST "$EDGE/v1/replay" -H 'content-type: application/json' \
  -d "{\"events\":$EVS,\"rate_per_sec\":1000,\"run_id\":\"smoke-r3\"}" >/dev/null
for _ in $(seq 1 50); do
  st="$(curl_edge "$EDGE/v1/replay" | python3 -c 'import json,sys; print(json.load(sys.stdin)["state"])')"
  [[ "$st" == "done" ]] && break
  sleep 0.2
done
[[ "$st" == "done" ]] || { echo "FAIL: replay did not finish ($st)"; exit 1; }
curl_edge "$EDGE/v1/journal/stats?origin=replay" |
  python3 -c 'import json,sys; d=json.load(sys.stdin)["stats"]; assert d["count"]==4 and d["by_type"]["smoke.a"]==2, d; print("journal stats ok")'
curl_edge -X POST "$EDGE/v1/journal/resend" -H 'content-type: application/json' \
  -d '{"seq":1,"severity":"critical","set":{"i":99,"extra.flag":true},"remove":["drop"],"idempotency_key":"smoke-resend"}' |
  python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["data"]=={"i":99,"extra":{"flag":True}} and d["results"][0]["idempotency_key"]=="smoke-resend", d; print("journal resend ok")'
curl_edge -X POST "$EDGE/v1/journal/diff" -H 'content-type: application/json' \
  -d "{\"expected\":$EVS,\"origin\":\"replay\"}" >/dev/null
curl_edge -X POST "$EDGE/v1/chaos/config" -H 'content-type: application/json' -d '{"enabled":true,"seed":1,"drop_pct":20,"correlation_pct":80,"types":["fleet.*"]}' >/dev/null
curl_edge -X POST "$EDGE/v1/chaos/reset" >/dev/null

echo "PASS: pipeline smoke"
