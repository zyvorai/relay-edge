#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Full event matrix: farm · firewater/edge · remote-edge · fleet through relay-pubsub → Relay.
#
# Usage:
#   BASE=https://127.0.0.1:8443 GATEWAY=https://127.0.0.1:8081 EDGE=http://127.0.0.1:18086 \
#     ./scripts/e2e-events-matrix.sh
#
# Requires: Relay (BASE), relay-pubsub relay-events (GATEWAY, curl -k), relay-edge (EDGE).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib/relay-api.sh
source "$SCRIPT_DIR/lib/relay-api.sh"

PROJECT="${PROJECT:-fasal-onprem}"
FAILED=0

pass() { echo "  ✅ $1"; }
fail() { echo "  ❌ $1" >&2; FAILED=$((FAILED + 1)); }

relay_api_init
echo "== relay-edge event matrix — relay=$BASE gateway=$GATEWAY edge=$EDGE =="
"${CURL_RELAY[@]}" "$BASE/healthz" >/dev/null
"${CURL_GW[@]}" "$GATEWAY/healthz" >/dev/null
curl -fsSk "$EDGE/healthz" | python3 -c '
import json,sys
d=json.load(sys.stdin)
mods=set(d.get("modules") or [])
for m in ("firewater","fleet"):
    assert m in mods, mods
sim = "remote-edge" if "remote-edge" in mods else "atlas" if "atlas" in mods else None
assert sim, mods
print("edge modules ok:", ",".join(sorted(mods)), "sim=", sim)
print(sim, file=open("/tmp/relay-edge-sim-module","w"))
'
EDGE_SIM=$(cat /tmp/relay-edge-sim-module 2>/dev/null || echo remote-edge)

relay_api_login

wait_new_type() {
  local typ=$1 before=$2 got
  got=$(relay_api_wait_new_type "$typ" "$before")
  if [[ -z "$got" ]]; then
    fail "Relay never saw new event type=$typ"
    return 1
  fi
  echo "$got"
}

# ── A. Farm (via gateway REST, same contract as relay-pubsub fasal-catalog-smoke) ──
echo ""
echo "== A. Farm catalog (10 types) =="
FASAL_SCRIPT="$(cd "$SCRIPT_DIR/../.." && pwd)/relay-pubsub/scripts/fasal-catalog-smoke.sh"
if [[ -x "$FASAL_SCRIPT" ]]; then
  if BASE="$BASE" GATEWAY="$GATEWAY" PROJECT="$PROJECT" bash "$FASAL_SCRIPT"; then
    pass "farm 10/10 Accept + 5/5 Act via fasal-catalog-smoke"
  else
    fail "farm catalog smoke failed (Accept/Act — check RELAY_ACTION_TARGETS + gateway TLS SAN includes 127.0.0.1)"
  fi
else
  echo "  (skip detailed farm — $FASAL_SCRIPT not found; run from sibling relay-pubsub checkout)"
fi

# ── B. Firewater / edge (via relay-edge publish) ──
echo ""
echo "== B. Firewater / edge =="
curl -fsSk -X POST "$EDGE/v1/firewater/seed" >/dev/null
curl -fsSk -X POST "$EDGE/v1/firewater/config" -H 'content-type: application/json' \
  -d '{"publish":true,"telemetry_always":false,"interval_ms":5000}' >/dev/null

FW_CASES=(
  "lowtank:firewater.tank.low"
  "fire:firewater.demand.active"
  "comms:edge.comms.down"
  "vision:edge.vision.fire"
  "gas:edge.gas.alarm"
)
for row in "${FW_CASES[@]}"; do
  scen="${row%%:*}"
  want="${row#*:}"
  before=$(relay_api_ids_for_type "$want" | tr '\n' ' ')
  curl -fsSk -X POST "$EDGE/v1/firewater/scenario" -H 'content-type: application/json' \
    -d "{\"scenario\":\"$scen\"}" >/dev/null
  got=$(wait_new_type "$want" "$before" || true)
  if [[ -n "$got" ]]; then
    pass "firewater $scen → $want (${got%% *})"
  fi
done

# ── C. Remote edge (or legacy atlas on older deploys) ──
echo ""
echo "== C. Remote edge ($EDGE_SIM) =="
curl -fsSk -X POST "$EDGE/v1/$EDGE_SIM/config" -H 'content-type: application/json' -d '{"publish":true}' >/dev/null
REMOTE_EDGE_CASES=(
  "sat_down:${EDGE_SIM}.link.starlink.degraded"
  "offline:${EDGE_SIM}.link.offline"
  "gpu_hot:${EDGE_SIM}.galleon.thermal"
  "intrusion:${EDGE_SIM}.vision.intrusion"
  "flood:${EDGE_SIM}.iot.flood"
  "drone_patrol:${EDGE_SIM}.uav.rtb"
)
for row in "${REMOTE_EDGE_CASES[@]}"; do
  scen="${row%%:*}"
  want="${row#*:}"
  before=$(relay_api_ids_for_type "$want" | tr '\n' ' ')
  curl -fsSk -X POST "$EDGE/v1/$EDGE_SIM/scenario" -H 'content-type: application/json' \
    -d "{\"scenario\":\"$scen\"}" >/dev/null
  got=$(wait_new_type "$want" "$before" || true)
  if [[ -n "$got" ]]; then
    pass "$EDGE_SIM $scen → $want (${got%% *})"
  fi
done

# ── D. Fleet master catalog ──
echo ""
echo "== D. Fleet =="
curl -fsSk -X POST "$EDGE/v1/fleet/config" -H 'content-type: application/json' -d '{"publish":true}' >/dev/null
FLEET_CASES=(
  "blackout:fleet.power.island"
  "amr_lost:fleet.robot.lost"
  "ot_storm:fleet.ot.ids"
  "spill:fleet.env.exceedance"
  "heatwave:fleet.dc.thermal"
  "intrusion:fleet.access.fault"
)
for row in "${FLEET_CASES[@]}"; do
  scen="${row%%:*}"
  want="${row#*:}"
  before=$(relay_api_ids_for_type "$want" | tr '\n' ' ')
  curl -fsSk -X POST "$EDGE/v1/fleet/scenario" -H 'content-type: application/json' \
    -d "{\"scenario\":\"$scen\"}" >/dev/null
  got=$(wait_new_type "$want" "$before" || true)
  if [[ -n "$got" ]]; then
    pass "fleet $scen → $want (${got%% *})"
  fi
done

echo ""
if [[ "$FAILED" -gt 0 ]]; then
  echo "FAILED: $FAILED matrix row(s)" >&2
  exit 1
fi
echo "PASS: event matrix (farm + firewater/edge + remote-edge + fleet)"
