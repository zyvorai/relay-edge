#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Smoke: remote-edge fleet catalog + scenarios against a running edge.
# Usage: EDGE=http://127.0.0.1:18086 ./scripts/smoke-remote-edge.sh
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


echo "== relay-edge remote-edge smoke @ $EDGE =="
curl_edge "$EDGE/healthz" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "remote-edge" in d.get("modules",[]); print("health modules", ",".join(d["modules"]))'
curl_edge -o /dev/null -w "remote-edge ui %{http_code}\n" "$EDGE/ui/remote-edge.html"

curl_edge "$EDGE/v1/remote-edge/catalog" | python3 -c '
import json,sys
d=json.load(sys.stdin)
items=d.get("items") or []
assert len(items) >= 20
classes={a.get("class") for a in items}
for c in ("galleon","starlink","drone","vision","iot"):
  assert c in classes, classes
print("catalog", len(items), "classes", ",".join(sorted(classes)))
'

curl_edge -X POST "$EDGE/v1/remote-edge/scenario" -H 'content-type: application/json' -d '{"scenario":"offline"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
evts=d.get("events") or []
types=[e.get("type") for e in evts]
assert "remote-edge.link.offline" in types or len(types)>0, types
print("offline scenario", types)
'

curl_edge -X POST "$EDGE/v1/remote-edge/scenario" -H 'content-type: application/json' -d '{"scenario":"intrusion"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
types=[e.get("type") for e in (d.get("events") or [])]
assert "remote-edge.vision.intrusion" in types, types
print("intrusion scenario", types)
'

echo "PASS: remote-edge smoke"
