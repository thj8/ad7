#!/usr/bin/env bash
# demo.sh — query competitions, submit flags, check leaderboard
# Usage: BASE_URL=http://host:8080 ./scripts/demo.sh
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8080}"
SECRET="change-me-in-production"

# ── JWT generator (HS256 via openssl) ──────────────────────────────────────
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

USER_TOKEN=$(jwt "player_001" "user")
ADMIN_TOKEN=$(jwt "admin" "admin")

hdr_user=(-H "Authorization: Bearer $USER_TOKEN" -H "Content-Type: application/json")
hdr_admin=(-H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json")

pp() { python3 -m json.tool 2>/dev/null || cat; }

# ── 1. List active competitions ────────────────────────────────────────────
echo "=== Active competitions ==="
COMPS=$(curl -sf "$BASE/api/v1/competitions" "${hdr_user[@]}")
echo "$COMPS" | pp

# Pick first competition id
COMP_ID=$(echo "$COMPS" | python3 -c "
import sys,json
cs=json.load(sys.stdin)['competitions']
print(cs[0]['id'] if cs else '')
")

if [ -z "$COMP_ID" ]; then
  echo "No active competitions found. Run: go run ./cmd/seed/"
  exit 1
fi

echo ""
echo "=== Competition $COMP_ID ==="
curl -sf "$BASE/api/v1/competitions/$COMP_ID" "${hdr_user[@]}" | pp

# ── 2. List challenges in competition ─────────────────────────────────────
echo ""
echo "=== Challenges in competition $COMP_ID ==="
CHALS=$(curl -sf "$BASE/api/v1/competitions/$COMP_ID/challenges" "${hdr_user[@]}")
echo "$CHALS" | pp

# Pick first challenge id
CHAL_ID=$(echo "$CHALS" | python3 -c "
import sys,json
cs=json.load(sys.stdin)['challenges']
print(cs[0]['id'] if cs else '')
")

# ── 3. Submit a flag (wrong then correct) ─────────────────────────────────
if [ -n "$CHAL_ID" ]; then
  echo ""
  echo "=== Submit wrong flag for challenge $CHAL_ID ==="
  curl -sf -X POST "$BASE/api/v1/competitions/$COMP_ID/challenges/$CHAL_ID/submit" \
    "${hdr_user[@]}" -d '{"flag":"flag{wrong}"}' | pp

  # Fetch the real flag from DB to submit correctly
  REAL_FLAG=$(mysql -h 192.168.5.44 -u root -pasfdsfedarjeiowvgfsd ctf -sNe \
    "SELECT flag FROM challenges WHERE res_id=$CHAL_ID" 2>/dev/null || echo "")

  if [ -n "$REAL_FLAG" ]; then
    echo ""
    echo "=== Submit correct flag for challenge $CHAL_ID ==="
    curl -sf -X POST "$BASE/api/v1/competitions/$COMP_ID/challenges/$CHAL_ID/submit" \
      "${hdr_user[@]}" -d "{\"flag\":\"$REAL_FLAG\"}" | pp
  fi
fi

# ── 4. Leaderboard ────────────────────────────────────────────────────────
echo ""
echo "=== Leaderboard for competition $COMP_ID ==="
curl -sf "$BASE/api/v1/competitions/$COMP_ID/leaderboard" "${hdr_user[@]}" | pp

# ── 5. Notifications ──────────────────────────────────────────────────────
echo ""
echo "=== Create notification (admin) ==="
curl -sf -X POST "$BASE/api/v1/admin/competitions/$COMP_ID/notifications" \
  "${hdr_admin[@]}" \
  -d '{"title":"Demo notice","message":"This is a test notification"}' | pp

echo ""
echo "=== List notifications ==="
curl -sf "$BASE/api/v1/competitions/$COMP_ID/notifications" "${hdr_user[@]}" | pp

# ── 6. Admin: all competitions ────────────────────────────────────────────
echo ""
echo "=== All competitions (admin) ==="
curl -sf "$BASE/api/v1/admin/competitions" "${hdr_admin[@]}" | pp

echo ""
echo "Done."
