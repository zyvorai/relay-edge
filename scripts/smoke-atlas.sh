#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Smoke: Atlas-class fleet catalog + scenarios against a running edge.
# Usage: EDGE=http://127.0.0.1:18086 ./scripts/smoke-atlas.sh
set -euo pipefail
EDGE="${EDGE:-http://127.0.0.1:18086}"

echo "== relay-edge atlas smoke @ $EDGE =="
curl -fsS "$EDGE/healthz" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "atlas" in d.get("modules",[]); print("health modules", ",".join(d["modules"]))'
curl -fsS -o /dev/null -w "atlas ui %{http_code}\n" "$EDGE/ui/atlas.html"

curl -fsS "$EDGE/v1/atlas/catalog" | python3 -c '
import json,sys
d=json.load(sys.stdin)
items=d.get("items") or []
assert len(items) >= 20
classes={a.get("class") for a in items}
for c in ("galleon","starlink","drone","vision","iot"):
  assert c in classes, classes
print("catalog", len(items), "classes", ",".join(sorted(classes)))
'

curl -fsS -X POST "$EDGE/v1/atlas/scenario" -H 'content-type: application/json' -d '{"scenario":"offline"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
snap=d.get("snapshot") or d
assert snap.get("link_mode")=="offline", snap
types={e.get("type") for e in (d.get("events") or [])}
assert "atlas.link.offline" in types or len(types)>0, types
print("offline link", snap.get("link_mode"), "events", ",".join(sorted(types)[:6]))
'

curl -fsS -X POST "$EDGE/v1/atlas/scenario" -H 'content-type: application/json' -d '{"scenario":"intrusion"}' | python3 -c '
import json,sys
d=json.load(sys.stdin)
types={e.get("type") for e in (d.get("events") or [])}
assert "atlas.vision.intrusion" in types, types
print("intrusion ok")
'

echo "PASS: atlas smoke"
