#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Shared Relay + Forge helpers for relay-edge e2e scripts.
# Source from scripts: source "$(dirname "$0")/lib/relay-api.sh"

relay_api_init() {
  BASE="${BASE:-https://127.0.0.1:8443}"
  GATEWAY="${GATEWAY:-https://127.0.0.1:8081}"
  EDGE="${EDGE:-http://127.0.0.1:18086}"
  FORGE_BASE="${FORGE_BASE:-${RELAY_FORGE_BASE_URL:-}}"
  FORGE_API_KEY="${FORGE_API_KEY:-${RELAY_FORGE_API_KEY:-}}"
  USER="${RELAY_DEMO_USER:-demo}"
  PASS="${RELAY_DEMO_PASSWORD:-demo}"
  CURL_RELAY=(curl -fsSk)
  CURL_GW=(curl -k -fsS)
  RELAY_TOKEN="${RELAY_AUTH_TOKEN:-}"
  RELAY_AUTH=()
}

relay_api_login() {
  if [[ -n "$RELAY_TOKEN" ]]; then
    RELAY_AUTH=(-H "Authorization: Bearer $RELAY_TOKEN")
    return 0
  fi
  local login
  login=$("${CURL_RELAY[@]}" -X POST "$BASE/v1/auth/login" -H 'content-type: application/json' \
    -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
  RELAY_TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' <<<"$login")
  RELAY_AUTH=(-H "Authorization: Bearer $RELAY_TOKEN")
}

relay_api_wait_state() {
  local eid=$1 want=$2 secs=${3:-25} st=""
  for _ in $(seq 1 "$secs"); do
    sleep 1
    st=$(curl -fsSk "$BASE/v1/events/$eid" "${RELAY_AUTH[@]}" \
      | python3 -c 'import json,sys; print(json.load(sys.stdin)["event"]["state"])')
    case ",$want," in *",$st,"*) echo "$st"; return 0 ;; esac
  done
  echo "$st"
  return 1
}

relay_api_get_policy() {
  local pol_id=$1
  curl -fsSk "$BASE/v1/policies/$pol_id" "${RELAY_AUTH[@]}"
}

relay_api_put_policy_backend() {
  local pol_id=$1 backend=$2
  local body
  body=$(relay_api_get_policy "$pol_id" | python3 -c "
import json, sys, time
raw = json.load(sys.stdin)
raw['decision_backend'] = sys.argv[1]
raw['updated_at'] = time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())
print(json.dumps(raw))
" "$backend")
  curl -fsSk -X PUT "$BASE/v1/policies/$pol_id" "${RELAY_AUTH[@]}" \
    -H 'content-type: application/json' -d "$body" >/dev/null
}

relay_api_event_by_key() {
  local key=$1
  "${CURL_RELAY[@]}" "$BASE/v1/events?limit=200" "${RELAY_AUTH[@]}" | python3 -c "
import json, sys
key = sys.argv[1]
for e in json.load(sys.stdin).get('items') or []:
    if e.get('idempotency_key') == key:
        print(e['id'])
        break
" "$key"
}

relay_api_forge_decision_id() {
  local eid=$1
  curl -fsSk "$BASE/v1/events/$eid" "${RELAY_AUTH[@]}" | python3 -c '
import json, sys
e = json.load(sys.stdin)["event"]
print(e.get("forge_decision_record_id") or (e.get("tags") or {}).get("forge_decision_record_id", ""))
'
}

relay_api_forge_freeze() {
  local dr=$1 decision=$2 rationale=$3
  curl -fsS -m 15 -X POST "$FORGE_BASE/api/zeus/decisions/$dr/freeze" \
    -H "Authorization: Bearer $FORGE_API_KEY" -H 'content-type: application/json' \
    -d "{\"rationale\":\"$rationale\",\"decision\":\"$decision\"}"
}

relay_api_forge_probe() {
  curl -fsS -m 8 -X POST "$FORGE_BASE/api/zeus/decisions" \
    -H "Authorization: Bearer $FORGE_API_KEY" -H 'content-type: application/json' \
    -d "{\"agent\":\"zyvor-relay\",\"origin\":\"Investigation\",\"summary\":\"probe\",\"recommendationText\":\"noop\",\"evidence\":[{\"kind\":\"probe\"}],\"clientRequestId\":\"relay/probe/$(date +%s)\"}"
}
