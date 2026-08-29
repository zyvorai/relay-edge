#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Smoke: site → zone → contact → device → season → stage → advisory → irrigation.
# Usage: EDGE=http://127.0.0.1:18086 ./scripts/smoke.sh
#        EDGE=http://<HOST>:18086 ./scripts/smoke.sh
set -euo pipefail
EDGE="${EDGE:-https://127.0.0.1:18086}"
TS=$(date +%s)

echo "== relay-edge smoke @ $EDGE =="
curl -fsSk "$EDGE/healthz" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="ok"; print("health", ",".join(d.get("modules",[])))'

SITE="site-smoke-$TS"
ZONE="zone-smoke-$TS"
DEV="dev-smoke-$TS"
CONTACT="contact-smoke-$TS"
SEASON="season-smoke-$TS"

curl -fsSk -X POST "$EDGE/v1/sites" -H 'content-type: application/json' -d "{
  \"id\": \"$SITE\", \"name\": \"Farm Smoke $TS\", \"labels\": {\"region\": \"karnataka\"}
}" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["id"]; print("site", d["id"])'

curl -fsSk -X POST "$EDGE/v1/sites/$SITE/zones" -H 'content-type: application/json' -d "{
  \"id\": \"$ZONE\", \"name\": \"Block A4\", \"code\": \"A4\"
}" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["code"]=="A4"; print("zone", d["id"], d["code"])'

curl -fsSk -X PUT "$EDGE/v1/zones/$ZONE/telemetry" -H 'content-type: application/json' -d '{
  "url": "http://127.0.0.1:18091/v1/telemetry/A4",
  "json_path": "$.state",
  "expect": "irrigating"
}' | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["telemetry"]["url"]; print("telemetry", d["telemetry"]["url"])'

curl -fsSk -X POST "$EDGE/v1/contacts" -H 'content-type: application/json' -d "{
  \"id\": \"$CONTACT\",
  \"name\": \"Farmer Smoke\",
  \"role\": \"farmer\",
  \"fcm_token\": \"fcm-smoke-$TS\",
  \"sms\": \"+910000000001\",
  \"email\": \"farmer-smoke@lab.local\"
}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("contact", d["id"])'

curl -fsSk -X PUT "$EDGE/v1/sites/$SITE/routing" -H 'content-type: application/json' -d "{
  \"routing\": {\"farmer\": \"$CONTACT\", \"operator\": \"$CONTACT\"}
}" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["routing"]["farmer"]; print("routing", d["routing"])'

curl -fsSk -X POST "$EDGE/v1/devices" -H 'content-type: application/json' -d "{
  \"id\": \"$DEV\",
  \"zone_id\": \"$ZONE\",
  \"name\": \"Valve A4\",
  \"kind\": \"fasaljet\",
  \"external_id\": \"fj-smoke-$TS\",
  \"commands\": [\"irrigation.start\"]
}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("device", d["id"], d["external_id"])'

curl -fsSk -X POST "$EDGE/v1/seasons" -H 'content-type: application/json' -d "{
  \"id\": \"$SEASON\",
  \"name\": \"Kharif Smoke $TS\",
  \"crop\": \"grape\",
  \"site_id\": \"$SITE\",
  \"stage\": \"sowing\",
  \"status\": \"planned\",
  \"starts_at\": \"2026-06-01T00:00:00Z\",
  \"ends_at\": \"2026-10-31T00:00:00Z\",
  \"labels\": {\"region\": \"karnataka\"}
}" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["site_id"]; print("season", d["id"], "site=", d.get("site"), "stage=", d.get("stage"))'

curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/open" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["season"]["status"]=="active"; print("opened", d["publish"]["path"])'

curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/stage" -H 'content-type: application/json' -d '{"stage":"vegetative"}' \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["season"]["stage"]=="vegetative"; print("stage", d["season"]["stage"], d.get("publish",{}).get("path"))'

curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/advisories" -H 'content-type: application/json' -d '{
  "type": "spray.advisory",
  "severity": "info",
  "message": "Spray window open for next 6h"
}' | python3 -c 'import json,sys; d=json.load(sys.stdin); print("advisory", d["publish"]["path"])'

curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/events" -H 'content-type: application/json' -d "{
  \"type\": \"irrigation.required\",
  \"severity\": \"critical\",
  \"command\": \"irrigation.start\",
  \"zone_id\": \"$ZONE\",
  \"device_id\": \"$DEV\",
  \"data\": {\"duration_minutes\": 12}
}" | python3 -c '
import json,sys
d=json.load(sys.stdin)
st=d.get("stamped") or {}
assert st.get("zone")=="A4" or st.get("zone_id"), st
assert st.get("fasal_device_id"), st
print("event publish", d["publish"]["path"], "stamped", st)
'

curl -fsSk "$EDGE/v1/seasons" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("seasons=", len(d["items"]))'
curl -fsSk "$EDGE/v1/sites" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("sites=", len(d["items"]))'
echo "PASS: relay-edge smoke"
