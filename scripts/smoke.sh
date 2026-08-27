#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Smoke: create season → open → publish irrigation → list.
# Usage: EDGE=http://212.8.248.187:18086 ./scripts/smoke.sh
set -euo pipefail
EDGE="${EDGE:-http://127.0.0.1:18086}"

echo "== relay-edge smoke @ $EDGE =="
curl -fsS "$EDGE/healthz" >/dev/null

ID="season-smoke-$(date +%s)"
curl -fsS -X POST "$EDGE/v1/seasons" -H 'content-type: application/json' -d "{
  \"id\": \"$ID\",
  \"name\": \"Kharif Smoke $ID\",
  \"crop\": \"grape\",
  \"site\": \"Farm-184\",
  \"status\": \"planned\",
  \"starts_at\": \"2026-06-01T00:00:00Z\",
  \"ends_at\": \"2026-10-31T00:00:00Z\",
  \"labels\": {\"region\": \"karnataka\"}
}" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["id"]; print("created", d["id"], d["status"])'

curl -fsS -X POST "$EDGE/v1/seasons/$ID/open" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["season"]["status"]=="active"; print("opened", d["publish"]["path"])'

curl -fsS -X POST "$EDGE/v1/seasons/$ID/events" -H 'content-type: application/json' -d '{
  "type": "irrigation.required",
  "severity": "critical",
  "command": "irrigation.start",
  "zone": "A4",
  "data": {"duration_minutes": 12}
}' | python3 -c 'import json,sys; d=json.load(sys.stdin); print("event publish", d["publish"]["path"], d.get("publish",{}).get("raw","")[:80])'

curl -fsS "$EDGE/v1/seasons" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("seasons=", len(d["items"]))'
echo "PASS: relay-edge smoke"
