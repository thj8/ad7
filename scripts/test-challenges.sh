#!/usr/bin/env bash
# test-challenges.sh — Interactive menu for Challenge API
# Usage: BASE_URL=http://host:8080 ./scripts/test-challenges.sh
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

CHAL_ID=""

pick_chal() {
  if [ -n "$CHAL_ID" ]; then return 0; fi
  local CHALS
  CHALS=$(curl -sf "$BASE/api/v1/challenges" "${hdr_user[@]}")
  CHAL_ID=$(echo "$CHALS" | python3 -c "
import sys,json
cs=json.load(sys.stdin)['challenges']
print(cs[0]['id'] if cs else '')
")
  if [ -z "$CHAL_ID" ]; then
    echo "No challenges found. Create one first (option 2)."
    return 1
  fi
  echo "Using challenge: $CHAL_ID"
}

do_list() {
  echo "=== List challenges (user) ==="
  curl -sf "$BASE/api/v1/challenges" "${hdr_user[@]}" | pp
}

do_create() {
  echo "=== Create challenge (admin) ==="
  local RESULT
  RESULT=$(curl -sf -X POST "$BASE/api/v1/admin/challenges" "${hdr_admin[@]}" \
    -d '{"title":"Test Challenge","category":"misc","description":"A test challenge","score":100,"flag":"flag{test_flag_123}"}')
  echo "$RESULT" | pp
  CHAL_ID=$(echo "$RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
  echo "Created: $CHAL_ID"
}

do_get() {
  pick_chal || return
  echo "=== Get challenge detail ==="
  curl -sf "$BASE/api/v1/challenges/$CHAL_ID" "${hdr_user[@]}" | pp
}

do_update() {
  pick_chal || return
  echo "=== Update challenge ==="
  curl -sf -X PUT "$BASE/api/v1/admin/challenges/$CHAL_ID" "${hdr_admin[@]}" \
    -d '{"title":"Updated Challenge","category":"web","description":"Updated","score":200,"flag":"flag{updated}","is_enabled":true}'
  echo "(204 = success)"
}

do_delete() {
  pick_chal || return
  echo "=== Delete challenge ==="
  curl -sf -X DELETE "$BASE/api/v1/admin/challenges/$CHAL_ID" "${hdr_admin[@]}"
  echo "(204 = success)"
  CHAL_ID=""
}

menu() {
  echo ""
  echo "============================================"
  echo "  Challenge API  [ID: ${CHAL_ID:-(none)}]"
  echo "============================================"
  echo "  1) List challenges (user)"
  echo "  2) Create challenge (admin)"
  echo "  3) Get challenge detail"
  echo "  4) Update challenge"
  echo "  5) Delete challenge"
  echo "  0) Exit"
  echo "============================================"
  read -rp "Choose [0-5]: " choice
  case "$choice" in
    1) do_list ;;
    2) do_create ;;
    3) do_get ;;
    4) do_update ;;
    5) do_delete ;;
    0) echo "Bye."; exit 0 ;;
    *) echo "Invalid choice." ;;
  esac
}

while true; do menu; done
