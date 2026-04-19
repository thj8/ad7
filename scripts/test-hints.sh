#!/usr/bin/env bash
# test-hints.sh — Interactive menu for Hints API
# Usage: BASE_URL=http://host:8080 ./scripts/test-hints.sh
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
HINT_ID=""

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
    echo "No challenges found. Run: go run ./cmd/seed/"
    return 1
  fi
  echo "Using challenge: $CHAL_ID"
}

do_list() {
  pick_chal || return
  echo "=== List hints ==="
  curl -sf "$BASE/api/v1/challenges/$CHAL_ID/hints" "${hdr_user[@]}" | pp
}

do_create() {
  pick_chal || return
  echo "=== Create hint (admin) ==="
  local RESULT
  RESULT=$(curl -sf -X POST "$BASE/api/v1/admin/challenges/$CHAL_ID/hints" \
    "${hdr_admin[@]}" \
    -d '{"content":"Try looking at the source code"}')
  echo "$RESULT" | pp
  HINT_ID=$(echo "$RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "")
  if [ -n "$HINT_ID" ]; then
    echo "Created hint: $HINT_ID"
  fi
}

do_update() {
  if [ -z "$HINT_ID" ]; then
    echo "No hint created yet. Use option 2 first."
    return
  fi
  echo "=== Update hint (admin) ==="
  curl -sf -X PUT "$BASE/api/v1/admin/hints/$HINT_ID" "${hdr_admin[@]}" \
    -d '{"content":"Updated hint: check the HTTP headers","is_visible":true}'
  echo "(204 = success)"
}

do_delete() {
  if [ -z "$HINT_ID" ]; then
    echo "No hint created yet. Use option 2 first."
    return
  fi
  echo "=== Delete hint (admin) ==="
  curl -sf -X DELETE "$BASE/api/v1/admin/hints/$HINT_ID" "${hdr_admin[@]}"
  echo "(204 = success)"
  HINT_ID=""
}

menu() {
  echo ""
  echo "============================================"
  echo "  Hints API"
  echo "  Chal: ${CHAL_ID:-(auto)}  Hint: ${HINT_ID:-(none)}"
  echo "============================================"
  echo "  1) List hints"
  echo "  2) Create hint (admin)"
  echo "  3) Update hint (admin)"
  echo "  4) Delete hint (admin)"
  echo "  0) Exit"
  echo "============================================"
  read -rp "Choose [0-4]: " choice
  case "$choice" in
    1) do_list ;;
    2) do_create ;;
    3) do_update ;;
    4) do_delete ;;
    0) echo "Bye."; exit 0 ;;
    *) echo "Invalid choice." ;;
  esac
}

while true; do menu; done
