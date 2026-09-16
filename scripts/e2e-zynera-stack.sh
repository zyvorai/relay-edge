#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
# Full stack: relay-edge event matrix + Zynera Decision Record approval path.
#
# Usage:
#   set -a && source config/lab-stack.env && set +a
#   ./scripts/e2e-zynera-stack.sh
#   ./scripts/e2e-zynera-stack.sh --skip-matrix   # zynera path only
#
# Requires on Relay (not relay-edge): RELAY_FORGE_BASE_URL + RELAY_FORGE_API_KEY
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/relay-api.sh
source "$SCRIPT_DIR/lib/relay-api.sh"

SKIP_MATRIX=0
for arg in "$@"; do
  case "$arg" in
    --skip-matrix) SKIP_MATRIX=1 ;;
    -h|--help)
      echo "Usage: $0 [--skip-matrix]"
      echo "Env: BASE GATEWAY EDGE ZYNERA_BASE ZYNERA_API_KEY RELAY_AUTH_TOKEN"
      exit 0
      ;;
  esac
done

relay_api_init
FAILED=0
SKIPPED=0
pass() { echo "  ✅ $1"; }
skip() { echo "  ⚠️  $1"; SKIPPED=$((SKIPPED + 1)); }
fail() { echo "  ❌ $1" >&2; FAILED=$((FAILED + 1)); }

POL_RESTORE=""
restore_policy() {
  if [[ -n "$POL_RESTORE" && -f "$POL_RESTORE" ]]; then
    relay_api_login 2>/dev/null || true
    curl -fsSk -X PUT "$BASE/v1/policies/pol_critical_farm" "${RELAY_AUTH[@]}" \
      -H 'content-type: application/json' -d @"$POL_RESTORE" >/dev/null 2>&1 || true
    rm -f "$POL_RESTORE"
  fi
}
trap restore_policy EXIT

echo "== e2e zynera stack — relay=$BASE gateway=$GATEWAY edge=$EDGE =="

# ── Phase A: event matrix ──
if [[ "$SKIP_MATRIX" -eq 0 ]]; then
  echo ""
  echo "== Phase A: event matrix =="
  if BASE="$BASE" GATEWAY="$GATEWAY" EDGE="$EDGE" bash "$SCRIPT_DIR/e2e-events-matrix.sh"; then
    pass "event matrix"
  else
    fail "event matrix"
  fi
else
  skip "event matrix (--skip-matrix)"
fi

# ── Phases B–G: Zynera path (skip if Zynera unset) ──
if [[ -z "${ZYNERA_BASE:-}" || -z "${ZYNERA_API_KEY:-}" ]]; then
  skip "Zynera path skipped (set ZYNERA_BASE + ZYNERA_API_KEY; RELAY_FORGE_* must be on Relay)"
  echo ""
  if [[ "$FAILED" -gt 0 ]]; then
    echo "FAILED: e2e-zynera-stack ($FAILED failure(s), skipped=$SKIPPED)" >&2
    exit 1
  fi
  echo "PASS: e2e-zynera-stack (Zynera optional — skipped=$SKIPPED)"
  exit 0
fi

if ! relay_api_zynera_probe >/dev/null 2>/tmp/zynera-probe.err; then
  skip "Zynera unreachable — $(head -c 100 /tmp/zynera-probe.err)"
  echo ""
  if [[ "$FAILED" -gt 0 ]]; then
    echo "FAILED: e2e-zynera-stack ($FAILED failure(s), skipped=$SKIPPED)" >&2
    exit 1
  fi
  echo "PASS: e2e-zynera-stack (Zynera optional — skipped=$SKIPPED)"
  exit 0
fi

relay_api_login

echo ""
echo "== Phase B: policy pol_critical_farm → decision_backend=forge =="
POL_RESTORE=$(mktemp)
relay_api_get_policy pol_critical_farm >"$POL_RESTORE"
relay_api_put_policy_backend pol_critical_farm forge
pass "policy patched (restored on exit)"

echo ""
echo "== Phase C: relay-edge publish irrigation.required =="
TS=$(date +%s)
SITE="site-zynera-$TS"
ZONE="zone-zynera-$TS"
DEV="dev-zynera-$TS"
CONTACT="contact-zynera-$TS"
SEASON="season-zynera-$TS"
IDEM_KEY="edge/zynera-stack/$TS/irrigation.required"

curl -fsSk -X POST "$EDGE/v1/sites" -H 'content-type: application/json' -d "{
  \"id\": \"$SITE\", \"name\": \"Zynera Stack $TS\"
}" >/dev/null
curl -fsSk -X POST "$EDGE/v1/sites/$SITE/zones" -H 'content-type: application/json' -d "{
  \"id\": \"$ZONE\", \"name\": \"Block A4\", \"code\": \"A4\"
}" >/dev/null
curl -fsSk -X PUT "$EDGE/v1/zones/$ZONE/telemetry" -H 'content-type: application/json' -d '{
  "url": "http://127.0.0.1:18091/v1/telemetry/A4",
  "json_path": "$.state",
  "expect": "irrigating"
}' >/dev/null
curl -fsSk -X POST "$EDGE/v1/contacts" -H 'content-type: application/json' -d "{
  \"id\": \"$CONTACT\", \"name\": \"Farmer\", \"role\": \"farmer\",
  \"fcm_token\": \"fcm-zynera-$TS\"
}" >/dev/null
curl -fsSk -X PUT "$EDGE/v1/sites/$SITE/routing" -H 'content-type: application/json' \
  -d "{\"routing\": {\"farmer\": \"$CONTACT\"}}" >/dev/null
curl -fsSk -X POST "$EDGE/v1/devices" -H 'content-type: application/json' -d "{
  \"id\": \"$DEV\", \"zone_id\": \"$ZONE\", \"name\": \"Valve\",
  \"kind\": \"fasaljet\", \"external_id\": \"fj-zynera-$TS\",
  \"commands\": [\"irrigation.start\"]
}" >/dev/null
curl -fsSk -X POST "$EDGE/v1/seasons" -H 'content-type: application/json' -d "{
  \"id\": \"$SEASON\", \"name\": \"Zynera Stack Season\", \"crop\": \"grape\",
  \"site_id\": \"$SITE\", \"status\": \"planned\"
}" >/dev/null
curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/open" >/dev/null

PUBLISH=$(curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/events" -H 'content-type: application/json' -d "{
  \"type\": \"irrigation.required\",
  \"severity\": \"critical\",
  \"command\": \"irrigation.start\",
  \"zone_id\": \"$ZONE\",
  \"device_id\": \"$DEV\",
  \"idempotency_key\": \"$IDEM_KEY\",
  \"data\": {\"duration_minutes\": 12}
}")
echo "$PUBLISH" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("publish"), d; print("publish", d["publish"].get("path"))'
pass "relay-edge published irrigation.required"

echo ""
echo "== Phase D: Relay ack → approve → awaiting_decision =="
EID=""
for _ in $(seq 1 30); do
  sleep 0.5
  EID=$(relay_api_event_by_key "$IDEM_KEY" || true)
  [[ -n "$EID" ]] && break
done
if [[ -z "$EID" ]]; then
  fail "Relay event not found for idempotency_key=$IDEM_KEY"
else
  pass "Relay event $EID"
  if ! st=$(relay_api_wait_state "$EID" "awaiting_ack,notifying,escalated" 20); then
    fail "stuck before ack (state=$st)"
  else
    code=$("${CURL_RELAY[@]}" -sS -o /tmp/ack-zynera-stack.json -w '%{http_code}' -X POST "$BASE/v1/events/$EID/ack" \
      "${RELAY_AUTH[@]}" -H 'content-type: application/json' \
      -d '{"decision":"approve","note":"e2e-zynera-stack"}' || echo 000)
    if [[ "$code" != "200" && "$code" != "201" ]]; then
      fail "approve HTTP $code $(head -c 200 /tmp/ack-zynera-stack.json)"
    elif ! st=$(relay_api_wait_state "$EID" "awaiting_decision" 15); then
      fail "expected awaiting_decision (state=$st) — is RELAY_FORGE_BASE_URL set on Relay?"
    else
      pass "awaiting_decision"
      DR=$(relay_api_zynera_decision_id "$EID")
      if [[ -z "$DR" ]]; then
        fail "missing forge_decision_record_id on $EID"
      else
        echo ""
        echo "== Phase E–F: Zynera freeze Approved → Relay act =="
        pass "Zynera Decision Record $DR"
        if relay_api_zynera_freeze "$DR" "Approved" "e2e-zynera-stack approve" >/dev/null; then
          if st=$(relay_api_wait_state "$EID" "verified,verifying,action_executed,action_pending" 35); then
            pass "zynera Approved → $st"
          else
            fail "post-freeze state=$st (check RELAY_ACTION_TARGETS + gateway)"
          fi
        else
          fail "zynera freeze failed for $DR"
        fi

        echo ""
        echo "== Phase G: Zynera freeze Rejected → failed =="
        IDEM_REJ="edge/zynera-stack-rej/$TS/irrigation.required"
        curl -fsSk -X POST "$EDGE/v1/seasons/$SEASON/events" -H 'content-type: application/json' -d "{
          \"type\": \"irrigation.required\",
          \"severity\": \"critical\",
          \"command\": \"irrigation.start\",
          \"zone_id\": \"$ZONE\",
          \"device_id\": \"$DEV\",
          \"idempotency_key\": \"$IDEM_REJ\",
          \"data\": {\"duration_minutes\": 5}
        }" >/dev/null
        EID_REJ=""
        for _ in $(seq 1 30); do
          sleep 0.5
          EID_REJ=$(relay_api_event_by_key "$IDEM_REJ" || true)
          [[ -n "$EID_REJ" ]] && break
        done
        if [[ -z "$EID_REJ" ]]; then
          fail "reject-path event not found"
        else
          relay_api_wait_state "$EID_REJ" "awaiting_ack,notifying,escalated" 20 >/dev/null || true
          curl -fsSk -X POST "$BASE/v1/events/$EID_REJ/ack" "${RELAY_AUTH[@]}" \
            -H 'content-type: application/json' \
            -d '{"decision":"approve","note":"e2e-zynera-stack-reject"}' >/dev/null || true
          if relay_api_wait_state "$EID_REJ" "awaiting_decision" 15 >/dev/null; then
            DR_REJ=$(relay_api_zynera_decision_id "$EID_REJ")
            relay_api_zynera_freeze "$DR_REJ" "Rejected" "e2e-zynera-stack reject" >/dev/null || true
            if st=$(relay_api_wait_state "$EID_REJ" "failed" 20); then
              pass "zynera Rejected → failed"
            else
              fail "reject-path expected failed (state=$st)"
            fi
          else
            fail "reject-path never reached awaiting_decision"
          fi
        fi
      fi
    fi
  fi
fi

echo ""
if [[ "$FAILED" -gt 0 ]]; then
  echo "FAILED: e2e-zynera-stack ($FAILED failure(s), skipped=$SKIPPED)" >&2
  exit 1
fi
echo "PASS: e2e-zynera-stack (skipped=$SKIPPED)"
