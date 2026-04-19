#!/usr/bin/env bash
# test-submissions.sh — Interactive menu for Submission API
# Usage: BASE_URL=http://host:8080 ./scripts/test-submissions.sh
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
CHAL_ID=""

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

pick_chal() {
  if [ -n "$CHAL_ID" ]; then return 0; fi
  pick_comp || return
  local CHALS
  CHALS=$(curl -sf "$BASE/api/v1/competitions/$COMP_ID/challenges" "${hdr_user[@]}")
  CHAL_ID=$(echo "$CHALS" | python3 -c "
import sys,json
cs=json.load(sys.stdin)['challenges']
print(cs[0]['id'] if cs else '')
")
  if [ -z "$CHAL_ID" ]; then
    echo "No challenges in competition."
    return 1
  fi
  echo "Using challenge: $CHAL_ID"
}

do_wrong() {
  pick_chal || return
  echo "=== Submit wrong flag ==="
  curl -sf -X POST "$BASE/api/v1/competitions/$COMP_ID/challenges/$CHAL_ID/submit" \
    "${hdr_user[@]}" -d '{"flag":"flag{wrong}"}' | pp
}

do_correct() {
  pick_chal || return
  local REAL_FLAG
  REAL_FLAG=$(mysql -h 192.168.5.44 -u root -pasfdsfedarjeiowvgfsd ctf -sNe \
    "SELECT flag FROM challenges WHERE res_id='$CHAL_ID'" 2>/dev/null || echo "")
  if [ -z "$REAL_FLAG" ]; then
    echo "Could not fetch real flag from DB."
    return
  fi
  echo "=== Submit correct flag ==="
  curl -sf -X POST "$BASE/api/v1/competitions/$COMP_ID/challenges/$CHAL_ID/submit" \
    "${hdr_user[@]}" -d "{\"flag\":\"$REAL_FLAG\"}" | pp
}

do_repeat() {
  pick_chal || return
  local REAL_FLAG
  REAL_FLAG=$(mysql -h 192.168.5.44 -u root -pasfdsfedarjeiowvgfsd ctf -sNe \
    "SELECT flag FROM challenges WHERE res_id='$CHAL_ID'" 2>/dev/null || echo "")
  if [ -z "$REAL_FLAG" ]; then
    echo "Could not fetch real flag from DB."
    return
  fi
  echo "=== Submit again (already solved) ==="
  curl -sf -X POST "$BASE/api/v1/competitions/$COMP_ID/challenges/$CHAL_ID/submit" \
    "${hdr_user[@]}" -d "{\"flag\":\"$REAL_FLAG\"}" | pp
}

do_list() {
  pick_comp || return
  echo "=== List submissions for competition (admin) ==="
  curl -sf "$BASE/api/v1/admin/competitions/$COMP_ID/submissions" "${hdr_admin[@]}" | pp
}

menu() {
  echo ""
  echo "============================================"
  echo "  Submission API"
  echo "  Comp: ${COMP_ID:-(auto)}"
  echo "  Chal: ${CHAL_ID:-(auto)}"
  echo "============================================"
  echo "  1) Submit wrong flag"
  echo "  2) Submit correct flag (from DB)"
  echo "  3) Submit again (already solved)"
  echo "  4) List submissions (admin)"
  echo "  0) Exit"
  echo "============================================"
  read -rp "Choose [0-4]: " choice
  case "$choice" in
    1) do_wrong ;;
    2) do_correct ;;
    3) do_repeat ;;
    4) do_list ;;
    0) echo "Bye."; exit 0 ;;
    *) echo "Invalid choice." ;;
  esac
}

while true; do menu; done
