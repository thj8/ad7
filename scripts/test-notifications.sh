#!/usr/bin/env bash
# test-notifications.sh — Interactive menu for Notification API
# Usage: BASE_URL=http://host:8080 ./scripts/test-notifications.sh
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
    echo "No active competitions. Run: go run ./cmd/seed/"
    return 1
  fi
  echo "Using competition: $COMP_ID"
}

do_list() {
  pick_comp || return
  echo "=== List notifications ==="
  curl -sf "$BASE/api/v1/competitions/$COMP_ID/notifications" "${hdr_user[@]}" | pp
}

do_create() {
  pick_comp || return
  echo "=== Create notification (admin) ==="
  curl -sf -X POST "$BASE/api/v1/admin/competitions/$COMP_ID/notifications" \
    "${hdr_admin[@]}" \
    -d '{"title":"Test Notice","message":"This is a test notification"}'
  echo "(201 = success)"
}

menu() {
  echo ""
  echo "============================================"
  echo "  Notification API  [Comp: ${COMP_ID:-(auto)}]"
  echo "============================================"
  echo "  1) List notifications"
  echo "  2) Create notification (admin)"
  echo "  0) Exit"
  echo "============================================"
  read -rp "Choose [0-2]: " choice
  case "$choice" in
    1) do_list ;;
    2) do_create ;;
    0) echo "Bye."; exit 0 ;;
    *) echo "Invalid choice." ;;
  esac
}

while true; do menu; done
