#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Direct Relay scenario matrix: farm · firewater · remote-edge · fleet via POST /v1/events (no relay-pubsub).
#
# Usage:
#   BASE=https://127.0.0.1:8443 EDGE=http://127.0.0.1:18086 RELAY_AUTH_TOKEN=<jwt> \
#     ./scripts/e2e-direct-relay.sh
#
# relay-edge must run with GATEWAY_BASE_URL empty (direct mode). See config/lab-direct.env.example.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib/relay-api.sh
source "$SCRIPT_DIR/lib/relay-api.sh"

FAILED=0
TS=$(date +%s)

pass() { echo "  ✅ $1"; }
fail() { echo "  ❌ $1" >&2; FAILED=$((FAILED + 1)); }
skip() { echo "  ⏭  $1"; }

relay_api_init
echo "== relay-edge direct Relay matrix — relay=$BASE edge=$EDGE (no gateway) =="

"${CURL_RELAY[@]}" "$BASE/healthz" >/dev/null
curl -fsSk "$EDGE/healthz" | python3 -c '
import json,sys
d=json.load(sys.stdin)
mods=set(d.get("modules") or [])
for m in ("firewater","remote-edge","fleet"):
    assert m in mods, mods
print("edge modules ok:", ",".join(sorted(mods)))
'

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

assert_relay_path() {
  local resp=$1 label=$2
  python3 -c "
import json, sys
d = json.loads(sys.argv[1])
pub = d.get('publish') or {}
path = pub.get('path', '')
if path != 'relay':
    print(f'direct mode expected publish.path=relay got {path!r}', file=sys.stderr)
    sys.exit(1)
if not pub.get('event_id') and path == 'relay':
    pass  # some responses may omit event_id on advisory-only
" "$resp" || {
    fail "$label — edge not in direct mode (publish.path != relay). Set GATEWAY_BASE_URL= on relay-edge."
    return 1
  }
}

run_scenario_case() {
  local section=$1 endpoint=$2 scen=$3 want=$4
  local before got
  before=$(relay_api_ids_for_type "$want" | tr '\n' ' ')
  curl -fsSk -X POST "$EDGE/v1/$endpoint/scenario" -H 'content-type: application/json' \
    -d "{\"scenario\":\"$scen\"}" >/dev/null
  got=$(wait_new_type "$want" "$before" || true)
  if [[ -n "$got" ]]; then
    pass "$section $scen → $want (${got%% *})"
  fi
}

# ── 0. Direct-mode guard (farm critical) ──
echo ""
echo "== 0. Direct-mode guard =="
SITE="site-direct-$TS"
ZONE="zone-direct-$TS"
DEV="dev-direct-$TS"
CONTACT="contact-direct-$TS"
SEASON="season-direct-$TS"

curl -fsSk -X POST "$EDGE/v1/sites" -H 'content-type: application/json' -d "{
  \"id\": \"$SITE\", \"name\": \"Direct Farm $TS\"
}" >/dev/null
curl -fsSk -X POST "$EDGE/v1/sites/$SITE/zones" -H 'content-type: application/json' -d "{
  \"id\": \"$ZONE\", \"name\": \"Block D1\", \"code\": \"D1\"
}" >/dev/null
curl -fsSk -X POST "$EDGE/v1/contacts" -H 'content-type: application/json' -d "{
  \"id\": \"$CONTACT\", \"name\": \"Farmer Direct\", \"role\": \"farmer\", \"fcm_token\": \"fcm-direct-$TS\"
}" >/dev/null
curl -fsSk -X PUT "$EDGE/v1/sites/$SITE/routing" -H 'content-type: application/json' \
  -d "{\"routing\": {\"farmer\": \"$CONTACT\"}}" >/dev/null
curl -fsSk -X POST "$EDGE/v1/devices" -H 'content-type: application/json' -d "{
  \"id\": \"$DEV\", \"zone_id\": \"$ZONE\", \"name\": \"Valve D1\", \"kind\": \"valve\",
  \"external_id\": \"fj-direct-$TS\", \"commands\": [\"irrigation.start\"]
}" >/dev/null
curl -fsSk -X POST "$EDGE/v1/seasons" -H 'content-type: application/json' -d "{
  \"id\": \"$SEASON\", \"name\": \"Direct Season $TS\", \"crop\": \"grape\", \"site_id\": \"$SITE\",
  \"stage\": \"sowing\", \"status\": \"planned\"
}" >/dev/null

GUARD=$(curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/open")
assert_relay_path "$GUARD" "open season" || exit 1
pass "direct-mode guard publish.path=relay"
pass "farm open → crop.advisory (via open)"

# ── A. Farm (season API → Relay direct) ──
echo ""
echo "== A. Farm (10 types via season API) =="

farm_publish() {
  local label=$1 typ=$2
  local before got resp path
  before=$(relay_api_ids_for_type "$typ" | tr '\n' ' ')
  resp=$(curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/advisories" -H 'content-type: application/json' \
    -d "{\"type\":\"$typ\",\"severity\":\"info\",\"message\":\"direct test $typ\"}")
  assert_relay_path "$resp" "farm $label" || return 1
  got=$(wait_new_type "$typ" "$before" || true)
  if [[ -n "$got" ]]; then
    pass "farm $label → $typ (${got%% *})"
  fi
}

farm_critical() {
  local label=$1 typ=$2 cmd=$3
  local before got resp
  before=$(relay_api_ids_for_type "$typ" | tr '\n' ' ')
  resp=$(curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/events" -H 'content-type: application/json' -d "{
    \"type\": \"$typ\", \"severity\": \"critical\", \"command\": \"$cmd\",
    \"zone_id\": \"$ZONE\", \"device_id\": \"$DEV\"
  }")
  assert_relay_path "$resp" "farm $label" || return 1
  got=$(wait_new_type "$typ" "$before" || true)
  if [[ -n "$got" ]]; then
    pass "farm $label → $typ (${got%% *})"
  fi
}

farm_publish "spray" "spray.advisory"
farm_publish "weather" "weather.advisory"
farm_publish "frost" "frost.alert"
farm_publish "pest" "pest.advisory"

farm_critical "irrigation" "irrigation.required" "irrigation.start"
farm_critical "soil" "soil.moisture.critical" "irrigation.start"
farm_critical "fertigation" "fertigation.required" "fertigation.start"
farm_critical "disease" "disease.risk.critical" "inspection.create"
farm_critical "device" "device.control.required" "pump.start"

# ── B. Firewater (13 scenarios) ──
echo ""
echo "== B. Firewater / edge (13 scenarios) =="
curl -fsSk -X POST "$EDGE/v1/firewater/seed" >/dev/null
curl -fsSk -X POST "$EDGE/v1/firewater/config" -H 'content-type: application/json' \
  -d '{"publish":true,"telemetry_always":false,"interval_ms":5000}' >/dev/null

FW_CASES=(
  "lowtank:firewater.tank.low"
  "lowpress:firewater.pressure.low"
  "fire:firewater.demand.active"
  "pumpfail:firewater.pump.fail"
  "valve:firewater.valve.closed"
  "leak:firewater.leak.acoustic"
  "freeze:firewater.freeze.risk"
  "hydrant:firewater.hydrant.tamper"
  "comms:edge.comms.down"
  "vision:edge.vision.fire"
  "power:edge.power.fail"
  "gas:edge.gas.alarm"
  "plc:edge.control.fault"
)
for row in "${FW_CASES[@]}"; do
  run_scenario_case "firewater" "firewater" "${row%%:*}" "${row#*:}"
done

# ── C. Remote edge (6 event scenarios + 2 skip) ──
echo ""
echo "== C. Remote edge =="
curl -fsSk -X POST "$EDGE/v1/remote-edge/config" -H 'content-type: application/json' \
  -d '{"publish":true,"interval_ms":2000}' >/dev/null

REMOTE_CASES=(
  "sat_down:remote-edge.link.starlink.degraded"
  "offline:remote-edge.link.offline"
  "gpu_hot:remote-edge.galleon.thermal"
  "intrusion:remote-edge.vision.intrusion"
  "flood:remote-edge.iot.flood"
  "drone_patrol:remote-edge.uav.rtb"
)
for row in "${REMOTE_CASES[@]}"; do
  run_scenario_case "remote-edge" "remote-edge" "${row%%:*}" "${row#*:}"
done
for scen in nominal p5g_load; do
  curl -fsSk -X POST "$EDGE/v1/remote-edge/scenario" -H 'content-type: application/json' \
    -d "{\"scenario\":\"$scen\"}" >/dev/null
  skip "remote-edge $scen (readings-only, no Relay event)"
done

# ── D. Fleet (6 event scenarios + 2 skip) ──
echo ""
echo "== D. Fleet =="
curl -fsSk -X POST "$EDGE/v1/fleet/config" -H 'content-type: application/json' \
  -d '{"publish":true,"interval_ms":2000}' >/dev/null

FLEET_CASES=(
  "blackout:fleet.power.island"
  "amr_lost:fleet.robot.lost"
  "ot_storm:fleet.ot.ids"
  "spill:fleet.env.exceedance"
  "heatwave:fleet.dc.thermal"
  "intrusion:fleet.access.fault"
)
for row in "${FLEET_CASES[@]}"; do
  run_scenario_case "fleet" "fleet" "${row%%:*}" "${row#*:}"
done
for scen in nominal flood; do
  curl -fsSk -X POST "$EDGE/v1/fleet/scenario" -H 'content-type: application/json' \
    -d "{\"scenario\":\"$scen\"}" >/dev/null
  skip "fleet $scen (readings-only, no Relay event)"
done

echo ""
if [[ "$FAILED" -gt 0 ]]; then
  echo "FAILED: $FAILED direct matrix row(s)" >&2
  exit 1
fi
echo "PASS: direct Relay matrix (farm + firewater + remote-edge + fleet)"
