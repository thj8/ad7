#!/usr/bin/env bash
# test-competitions.sh — Interactive menu for Competition API
# Usage: BASE_URL=http://host:8080 ./scripts/test-competitions.sh
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8080}"
SECRET="${JWT_SECRET:-change-me-in-production}"

jwt() {
  local sub="$1" role="${2:-user}"
  local header payload sig
  header=$(printf '{"alg":"HS256","typ":"JWT"}' | openssl base64 -A | tr '+/' '-_' | tr -d '=')
  payload=$(printf '{"sub":"%s","role":"%s","exp":%d}' "$sub" "$role" $(($(date +%s)+3600)) \
    | openssl base64 -A | tr '+/' '-_' | tr -d '=')
  sig=$(printf '%s.%s' "$header" "$payload" \
    | openssl dgst -sha256 -hmac "$SECRET" -binary \
    | openssl base64 -A | tr '+/' '-_' | tr -d '=')
  printf '%s.%s.%s' "$header" "$payload" "$sig"
}

USER_TOKEN=$(jwt "test_user_001" "user")
ADMIN_TOKEN=$(jwt "admin" "admin")
hdr_user=(-H "Authorization: Bearer $USER_TOKEN" -H "Content-Type: application/json")
hdr_admin=(-H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json")
pp() { python3 -m json.tool 2>/dev/null || cat; }

# Shared state
COMP_ID=""

pick_comp() {
  if [ -n "$COMP_ID" ]; then return 0; fi
  local COMPS
  COMPS=$(curl -sf "$BASE/api/v1/competitions" "${hdr_user[@]}")
  COMP_ID=$(echo "$COMPS" | python3 -c "
import sys,json
cs=json.load(sys.stdin)['competitions']
print(cs[0]['id'] if cs else '')
")
  if [ -z "$COMP_ID" ]; then
    echo "No active competitions. Create one first (option 2)."
    return 1
  fi
  echo "Using competition: $COMP_ID"
}

do_list() {
  echo "=== List active competitions (user) ==="
  curl -sf "$BASE/api/v1/competitions" "${hdr_user[@]}" | pp
}

do_create() {
  echo "=== Create competition (admin) ==="
  local NOW END_TIME RESULT
  NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  END_TIME=$(date -u -v+7d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "+7 days" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null)
  RESULT=$(curl -sf -X POST "$BASE/api/v1/admin/competitions" "${hdr_admin[@]}" \
    -d "{\"title\":\"Test Competition\",\"description\":\"Created by test script\",\"start_time\":\"$NOW\",\"end_time\":\"$END_TIME\"}")
  echo "$RESULT" | pp
  COMP_ID=$(echo "$RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
  echo "Created: $COMP_ID"
}

do_get() {
  pick_comp || return
  echo "=== Get competition detail ==="
  curl -sf "$BASE/api/v1/competitions/$COMP_ID" "${hdr_user[@]}" | pp
}

do_update() {
  pick_comp || return
  echo "=== Update competition ==="
  curl -sf -X PUT "$BASE/api/v1/admin/competitions/$COMP_ID" "${hdr_admin[@]}" \
    -d '{"title":"Updated Test Competition","description":"Updated","is_active":true}'
  echo "(204 = success)"
}

do_list_all() {
  echo "=== List all competitions (admin) ==="
  curl -sf "$BASE/api/v1/admin/competitions" "${hdr_admin[@]}" | pp
}

do_start() {
  pick_comp || return
  echo "=== Start competition ==="
  curl -sf -X POST "$BASE/api/v1/admin/competitions/$COMP_ID/start" "${hdr_admin[@]}" | pp
}

do_end() {
  pick_comp || return
  echo "=== End competition ==="
  curl -sf -X POST "$BASE/api/v1/admin/competitions/$COMP_ID/end" "${hdr_admin[@]}" | pp
}

do_challenges() {
  pick_comp || return
  echo "=== List challenges in competition ==="
  curl -sf "$BASE/api/v1/competitions/$COMP_ID/challenges" "${hdr_user[@]}" | pp
}

do_delete() {
  pick_comp || return
  echo "=== Delete competition ==="
  curl -sf -X DELETE "$BASE/api/v1/admin/competitions/$COMP_ID" "${hdr_admin[@]}"
  echo "(204 = success)"
  COMP_ID=""
}

menu() {
  echo ""
  echo "============================================"
  echo "  Competition API  [ID: ${COMP_ID:-(none)}]"
  echo "============================================"
  echo "  1) List active competitions (user)"
  echo "  2) Create competition (admin)"
  echo "  3) Get competition detail"
  echo "  4) Update competition"
  echo "  5) List all competitions (admin)"
  echo "  6) Start competition"
  echo "  7) End competition"
  echo "  8) List challenges in competition"
  echo "  9) Delete competition"
  echo "  0) Exit"
  echo "============================================"
  read -rp "Choose [0-9]: " choice
  case "$choice" in
    1) do_list ;;
    2) do_create ;;
    3) do_get ;;
    4) do_update ;;
    5) do_list_all ;;
    6) do_start ;;
    7) do_end ;;
    8) do_challenges ;;
    9) do_delete ;;
    0) echo "Bye."; exit 0 ;;
    *) echo "Invalid choice." ;;
  esac
}

while true; do menu; done
