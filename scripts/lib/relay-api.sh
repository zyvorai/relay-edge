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
  if [[ "${RELAY_TLS_INSECURE:-}" == "1" || "$BASE" == https://* ]]; then
    CURL_RELAY=(curl -fsSk)
  else
    CURL_RELAY=(curl -fsS)
  fi
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
    st=$("${CURL_RELAY[@]}" "$BASE/v1/events/$eid" "${RELAY_AUTH[@]}" \
      | python3 -c 'import json,sys; print(json.load(sys.stdin)["event"]["state"])')
    case ",$want," in *",$st,"*) echo "$st"; return 0 ;; esac
  done
  echo "$st"
  return 1
}

relay_api_get_policy() {
  local pol_id=$1
  "${CURL_RELAY[@]}" "$BASE/v1/policies" "${RELAY_AUTH[@]}" | python3 -c "
import json, sys
want = sys.argv[1]
for p in json.load(sys.stdin).get('items') or []:
    if p.get('id') == want:
        print(json.dumps(p))
        break
else:
    sys.exit(1)
" "$pol_id"
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
  "${CURL_RELAY[@]}" "$BASE/v1/events/$eid" "${RELAY_AUTH[@]}" | python3 -c '
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

relay_api_ids_for_type() {
  local typ=$1
  "${CURL_RELAY[@]}" "$BASE/v1/events?limit=200" "${RELAY_AUTH[@]}" | python3 -c "
import json, sys
typ = sys.argv[1]
for e in json.load(sys.stdin).get('items') or []:
    if e.get('type') == typ:
        print(e['id'])
" "$typ"
}

# Wait until Relay Accepts a new event of typ not in the before-id list.
# Prints: event_id policy_id state (or empty on timeout).
relay_api_wait_new_type() {
  local typ=$1
  local before=$2
  local after found=""
  for _ in $(seq 1 20); do
    sleep 0.5
    after=$(relay_api_ids_for_type "$typ" | sort -u)
    while read -r id; do
      [[ -z "$id" ]] && continue
      if [[ " $before " != *" $id "* ]]; then
        found=$("${CURL_RELAY[@]}" "$BASE/v1/events?limit=200" "${RELAY_AUTH[@]}" | python3 -c "
import json, sys
want = sys.argv[1]
for e in json.load(sys.stdin).get('items') or []:
    if e.get('id') == want:
        print(e['id'], e.get('policy_id',''), e.get('state',''))
        break
" "$id")
        break 2
      fi
    done <<<"$after"
  done
  echo "$found"
}
