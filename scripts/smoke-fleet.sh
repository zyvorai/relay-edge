#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Smoke: fleet catalog + scenarios against a running edge.
# Usage: EDGE=http://127.0.0.1:18086 ./scripts/smoke-fleet.sh
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


echo "== relay-edge fleet smoke @ $EDGE =="
curl_edge "$EDGE/healthz" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "fleet" in d.get("modules",[]); print("health modules", ",".join(d["modules"]), "version", d.get("version",""))'
curl_edge -o /dev/null -w "fleet ui %{http_code}\n" "$EDGE/ui/fleet.html"

curl_edge "$EDGE/v1/fleet/catalog" | python3 -c '
import json,sys
d=json.load(sys.stdin)
items=d.get("items") or []
classes=d.get("classes") or []
assert len(items) >= 60, len(items)
assert len(classes) >= 15, classes
for c in ("robot", "energy", "ot_gw", "agri", "security"):
  assert c in classes, classes
print("catalog", len(items), "classes", len(classes))
'

curl_edge -X POST "$EDGE/v1/fleet/scenario" -H 'content-type: application/json' -d '{"scenario":"blackout"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
types=[e.get("type") for e in (d.get("events") or [])]
assert "fleet.power.island" in types, types
print("blackout scenario", types)
'

curl_edge -X POST "$EDGE/v1/fleet/scenario" -H 'content-type: application/json' -d '{"scenario":"amr_lost"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
types=[e.get("type") for e in (d.get("events") or [])]
assert "fleet.robot.lost" in types, types
print("amr_lost scenario", types)
'

echo "PASS: fleet smoke"
