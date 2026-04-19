#!/usr/bin/env bash
# test-analytics.sh — Interactive menu for Analytics API
# Usage: BASE_URL=http://host:8080 ./scripts/test-analytics.sh
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
hdr_user=(-H "Authorization: Bearer $USER_TOKEN" -H "Content-Type: application/json")
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

do_overview() {
  pick_comp || return
  echo "=== Analytics: Overview ==="
  curl -sf "$BASE/api/v1/competitions/$COMP_ID/analytics/overview" "${hdr_user[@]}" | pp
}

do_categories() {
  pick_comp || return
  echo "=== Analytics: Categories ==="
  curl -sf "$BASE/api/v1/competitions/$COMP_ID/analytics/categories" "${hdr_user[@]}" | pp
}

do_users() {
  pick_comp || return
  echo "=== Analytics: Users ==="
  curl -sf "$BASE/api/v1/competitions/$COMP_ID/analytics/users" "${hdr_user[@]}" | pp
}

do_challenges() {
  pick_comp || return
  echo "=== Analytics: Challenges ==="
  curl -sf "$BASE/api/v1/competitions/$COMP_ID/analytics/challenges" "${hdr_user[@]}" | pp
}

menu() {
  echo ""
  echo "============================================"
  echo "  Analytics API  [Comp: ${COMP_ID:-(auto)}]"
  echo "============================================"
  echo "  1) Overview"
  echo "  2) Categories"
  echo "  3) Users"
  echo "  4) Challenges"
  echo "  0) Exit"
  echo "============================================"
  read -rp "Choose [0-4]: " choice
  case "$choice" in
    1) do_overview ;;
    2) do_categories ;;
    3) do_users ;;
    4) do_challenges ;;
    0) echo "Bye."; exit 0 ;;
    *) echo "Invalid choice." ;;
  esac
}

while true; do menu; done
