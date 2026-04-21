#!/bin/bash
# test-topthree-leaderboard.sh — 测试 flag 提交、一二三血和排行榜
# 需要先启动 auth-server 和 server

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
AUTH_URL="${AUTH_URL:-http://localhost:8081}"
JWT_SECRET="${JWT_SECRET:-change-me-in-production}"

echo "=== 配置 ==="
echo "CTF URL: $BASE_URL"
echo "Auth URL: $AUTH_URL"
echo ""

# JWT 生成函数
jwt() {
  local sub="$1" role="${2:-user}"
  local header payload sig
  header=$(printf '{"alg":"HS256","typ":"JWT"}' | openssl base64 -A | tr '+/' '-_' | tr -d '=')
  payload=$(printf '{"sub":"%s","role":"%s","exp":%d}' "$sub" "$role" $(($(date +%s)+3600)) \
    | openssl base64 -A | tr '+/' '-_' | tr -d '=')
  sig=$(printf '%s.%s' "$header" "$payload" \
    | openssl dgst -sha256 -hmac "$JWT_SECRET" -binary \
    | openssl base64 -A | tr '+/' '-_' | tr -d '=')
  printf '%s.%s.%s' "$header" "$payload" "$sig"
}

# 获取第一个比赛和题目
echo "=== 获取测试数据 ==="
ADMIN_TOKEN=$(jwt "admin" "admin")
COMPS=$(curl -sf "$BASE_URL/api/v1/competitions" -H "Authorization: Bearer $ADMIN_TOKEN")
COMP_ID=$(echo "$COMPS" | python3 -c "import sys,json; cs=json.load(sys.stdin)['competitions']; print(cs[0]['id'] if cs else '')")
if [ -z "$COMP_ID" ]; then
  echo "错误：没有找到比赛！请先运行 ./scripts/quick-seed.sh"
  exit 1
fi
echo "比赛 ID: $COMP_ID"

CHALS=$(curl -sf "$BASE_URL/api/v1/competitions/$COMP_ID/challenges" -H "Authorization: Bearer $ADMIN_TOKEN")
CHAL1_ID=$(echo "$CHALS" | python3 -c "import sys,json; cs=json.load(sys.stdin)['challenges']; print(cs[0]['id'] if cs else '')")
echo "题目 ID: $CHAL1_ID"

# 从数据库获取真实 flag
echo ""
echo "=== 获取 flag ==="
DB_HOST="192.168.5.44"
DB_USER="root"
DB_PASS="asfdsfedarjeiowvgfsd"
DB_NAME="ctf"
FLAG=$(mysql -h "$DB_HOST" -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" -sNe "SELECT flag FROM challenges WHERE res_id='$CHAL1_ID'" 2>/dev/null)
echo "Flag: $FLAG"

# 提交函数
submit_flag() {
  local user="$1"
  local token=$(jwt "$user" "member")
  echo "--- $user 提交 flag ---"
  curl -sf -X POST "$BASE_URL/api/v1/competitions/$COMP_ID/challenges/$CHAL1_ID/submit" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "{\"flag\":\"$FLAG\"}" | python3 -m json.tool
  echo ""
}

# 查询 topthree
get_topthree() {
  local token=$(jwt "player1" "member")
  echo ""
  echo "=== 一二三血 ==="
  curl -sf "$BASE_URL/api/v1/topthree/competitions/$COMP_ID" \
    -H "Authorization: Bearer $token" | python3 -m json.tool
}

# 查询排行榜
get_leaderboard() {
  local token=$(jwt "player1" "member")
  echo ""
  echo "=== 排行榜 ==="
  curl -sf "$BASE_URL/api/v1/competitions/$COMP_ID/leaderboard" \
    -H "Authorization: Bearer $token" | python3 -m json.tool
}

# 开始测试
echo ""
echo "============================================"
echo "  开始测试"
echo "============================================"

# 清理之前的提交和 topthree 记录
echo "清理旧数据..."
mysql -h "$DB_HOST" -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" -e "DELETE FROM submissions; DELETE FROM topthree_records;" 2>/dev/null

# player1 提交（一血）
sleep 1
submit_flag "player1"
sleep 2
get_topthree

# player2 提交（二血）
sleep 1
submit_flag "player2"
sleep 2
get_topthree

# player3 提交（三血）
sleep 1
submit_flag "player3"
sleep 2
get_topthree

# player4 提交（不进入 topthree）
sleep 1
submit_flag "player4"
sleep 2
get_topthree

# 查看排行榜
get_leaderboard

echo ""
echo "============================================"
echo "  测试完成！"
echo "============================================"
