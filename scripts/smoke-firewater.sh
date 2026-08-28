#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Smoke: fire-water seed + tick + scenario against a running edge.
# Usage: EDGE=http://127.0.0.1:18086 ./scripts/smoke-firewater.sh
set -euo pipefail
EDGE="${EDGE:-http://127.0.0.1:18086}"

echo "== relay-edge firewater smoke @ $EDGE =="
curl -fsS "$EDGE/healthz" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "firewater" in d.get("modules",[]); print("health modules", ",".join(d["modules"]))'
curl -fsS -o /dev/null -w "ui %{http_code}\n" "$EDGE/ui/"

curl -fsS -X POST "$EDGE/v1/firewater/seed" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["season"]["id"]=="season_fw_watch"
assert d["season"]["status"]=="active"
assert d["zone"]["code"]=="FW-A"
assert len(d["devices"])>=10
print("seed", d["season"]["id"], "devices", len(d["devices"]))
'

curl -fsS -X POST "$EDGE/v1/firewater/config" -H 'content-type: application/json' -d '{"publish":false,"telemetry_always":true,"interval_ms":2000}' >/dev/null

curl -fsS -X POST "$EDGE/v1/firewater/scenario" -H 'content-type: application/json' -d '{"scenario":"lowtank"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["scenario"]=="lowtank"
assert d["values"]["tank_level"] < 40
print("lowtank", round(d["values"]["tank_level"],2), "%")
'

curl -fsS -X POST "$EDGE/v1/firewater/scenario" -H 'content-type: application/json' -d '{"scenario":"fire"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["pump_on"] is True
print("fire pump_on", d["pump_on"], "flow", round(d["values"]["header_lps"],1))
'

curl -fsS "$EDGE/v1/firewater/events" | python3 -c '
import json,sys
d=json.load(sys.stdin)
items=d.get("items") or []
types={e.get("type") for e in items}
assert "firewater.tank.low" in types or "firewater.demand.active" in types, types
print("events", len(items), "types", ",".join(sorted(types)[:8]))
'

curl -fsS "$EDGE/v1/sites/site_fw_plant" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("site", d["id"], d["labels"])'
echo "PASS: firewater smoke"
