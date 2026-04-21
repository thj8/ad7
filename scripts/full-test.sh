#!/bin/bash
# full-test.sh — 完整的端到端测试脚本
# 1. 启动 auth-server 和 server
# 2. 通过 auth API 注册所有用户
# 3. 通过 CTF API 创建比赛和题目
# 4. 测试 flag 提交、一二三血、排行榜

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# 配置
AUTH_URL="${AUTH_URL:-http://localhost:8081}"
CTF_URL="${CTF_URL:-http://localhost:8080}"
ADMIN_USER="admin"
ADMIN_PASS="admin123"
USER_PASS="password123"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() { echo -e "${GREEN}>>> $1${NC}"; }
warn() { echo -e "${YELLOW}!!! $1${NC}"; }
err() { echo -e "${RED}!!! $1${NC}"; }

# HTTP 请求辅助函数
post_json() {
  local url="$1"
  local data="$2"
  local token="${3:-}"
  shift 3
  local args=("-s" "-o" "-" "-w" "\n%{http_code}" "-X" "POST" "-H" "Content-Type: application/json")
  if [ -n "$token" ]; then
    args+=("-H" "Authorization: Bearer $token")
  fi
  args+=("--data-binary" "$data" "$url")
  local resp
  resp=$(curl "${args[@]}")
  # 分离 body 和 status code（最后一行）
  local body=$(echo "$resp" | sed '$d')
  local code=$(echo "$resp" | tail -n 1)
  echo "$body"
  return $((code >= 200 && code < 300 ? 0 : 1))
}

get_json() {
  local url="$1"
  local token="${2:-}"
  shift 2
  local args=("-s" "-X" "GET")
  if [ -n "$token" ]; then
    args+=("-H" "Authorization: Bearer $token")
  fi
  args+=("$url")
  curl "${args[@]}"
}

# 用户注册和登录
register_user() {
  local username="$1"
  local password="$2"
  local role="${3:-member}"
  log "注册用户: $username (role: $role)"
  post_json "$AUTH_URL/api/v1/register" \
    "$(printf '{"username":"%s","password":"%s","role":"%s"}' "$username" "$password" "$role")"
}

login_user() {
  local username="$1"
  local password="$2"
  log "登录用户: $username"
  resp=$(post_json "$AUTH_URL/api/v1/login" \
    "$(printf '{"username":"%s","password":"%s"}' "$username" "$password")")
  echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])"
}

# 清理
cleanup() {
  log "清理进程..."
  pkill -f "go run ./cmd/auth-server" 2>/dev/null || true
  pkill -f "go run ./cmd/server" 2>/dev/null || true
  sleep 1
}

cleanup

# 清理数据库
log "清理数据库..."
mysql -h 192.168.5.44 -u root -pasfdsfedarjeiowvgfsd ctf -e "
  DELETE FROM topthree_records;
  DELETE FROM submissions;
  DELETE FROM competition_challenges;
  DELETE FROM challenges;
  DELETE FROM competitions;
  DELETE FROM team_members;
  DELETE FROM users;
  DELETE FROM teams;
" 2>/dev/null

# 启动服务器
log "启动 auth-server..."
go run ./cmd/auth-server -config cmd/auth-server/config.yaml &
AUTH_PID=$!
sleep 4

log "启动 server..."
go run ./cmd/server -config config.yaml &
CTF_PID=$!
sleep 5

trap cleanup EXIT

# 检查服务器
log "检查服务器..."
if ! curl -s "$AUTH_URL/api/v1/register" -o /dev/null --connect-timeout 5; then
  err "auth-server 启动失败"
  exit 1
fi
if ! curl -s "$CTF_URL/api/v1/competitions" -o /dev/null --connect-timeout 5; then
  err "server 启动失败"
  exit 1
fi

# 注册用户
log "============================================"
log "  1. 注册用户"
log "============================================"
register_user "$ADMIN_USER" "$ADMIN_PASS" "admin" || warn "admin 可能已存在"
ADMIN_TOKEN=$(login_user "$ADMIN_USER" "$ADMIN_PASS")
echo "admin token: ${ADMIN_TOKEN:0:20}..."

USER_TOKENS=()
for i in 1 2 3 4; do
  username="player${i}"
  register_user "$username" "$USER_PASS" "member"
  token=$(login_user "$username" "$USER_PASS")
  USER_TOKENS+=("$token")
  echo "player${i} token: ${token:0:20}..."
done

# 创建比赛和题目
log ""
log "============================================"
log "  2. 创建比赛和题目"
log "============================================"

# 创建比赛
log "创建比赛..."
COMP_RESP=$(post_json "$CTF_URL/api/v1/admin/competitions" \
  '{"title":"端到端测试比赛","description":"用于完整API测试","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}' \
  "$ADMIN_TOKEN")
COMP_ID=$(echo "$COMP_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
if [ -z "$COMP_ID" ]; then
  err "创建比赛失败: $COMP_RESP"
  exit 1
fi
log "比赛 ID: $COMP_ID"

# 创建题目
CHAL_IDS=()
CHAL_FLAGS=()
for i in 1 2; do
  title="Web 题 $i"
  score=$((100 + i * 100))
  flag="flag{e2e_chal_${i}_$(openssl rand -hex 4)}"
  log "创建题目: $title ($score 分)"
  CHAL_RESP=$(post_json "$CTF_URL/api/v1/admin/challenges" \
    "$(printf '{"title":"%s","description":"测试题目","category":"web","score":%d,"flag":"%s"}' "$title" "$score" "$flag")" \
    "$ADMIN_TOKEN")
  CHAL_ID=$(echo "$CHAL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
  if [ -z "$CHAL_ID" ]; then
    err "创建题目失败: $CHAL_RESP"
    exit 1
  fi
  CHAL_IDS+=("$CHAL_ID")
  CHAL_FLAGS+=("$flag")
  echo "  题目 $i ID: $CHAL_ID"
  echo "  题目 $i flag: $flag"
done

# 将题目添加到比赛
for CHAL_ID in "${CHAL_IDS[@]}"; do
  log "添加题目到比赛: $CHAL_ID"
  post_json "$CTF_URL/api/v1/admin/competitions/$COMP_ID/challenges" \
    "$(printf '{"challenge_id":"%s"}' "$CHAL_ID")" \
    "$ADMIN_TOKEN" >/dev/null
done

# 测试 flag 提交
log ""
log "============================================"
log "  3. 测试 Flag 提交"
log "============================================"

CHAL1_ID="${CHAL_IDS[0]}"
CHAL1_FLAG="${CHAL_FLAGS[0]}"
SUBMIT_PATH="/api/v1/competitions/$COMP_ID/challenges/$CHAL1_ID/submit"

submit_and_check() {
  local user_idx=$1
  local expected_msg=$2
  local username="player$((user_idx + 1))"
  local token="${USER_TOKENS[$user_idx]}"
  log "$username 提交 flag..."
  resp=$(post_json "$CTF_URL$SUBMIT_PATH" \
    "$(printf '{"flag":"%s"}' "$CHAL1_FLAG")" \
    "$token")
  echo "  响应: $resp"
  if echo "$resp" | grep -q "$expected_msg"; then
    echo -e "  ${GREEN}✓ 成功${NC}"
  else
    echo -e "  ${RED}✗ 失败${NC}"
  fi
}

# player1 提交 (一血)
sleep 1
submit_and_check 0 '"success":true'
sleep 2

# player2 提交 (二血)
sleep 1
submit_and_check 1 '"success":true'
sleep 2

# player3 提交 (三血)
sleep 1
submit_and_check 2 '"success":true'
sleep 2

# player4 提交 (不进入 top three)
sleep 1
submit_and_check 3 '"success":true'
sleep 2

# 查看一二三血
log ""
log "============================================"
log "  4. 查看一二三血"
log "============================================"
get_json "$CTF_URL/api/v1/topthree/competitions/$COMP_ID" "${USER_TOKENS[0]}" | python3 -m json.tool

# 查看排行榜
log ""
log "============================================"
log "  5. 查看排行榜"
log "============================================"
get_json "$CTF_URL/api/v1/competitions/$COMP_ID/leaderboard" "${USER_TOKENS[0]}" | python3 -m json.tool

# 让 player2 也解出第二题，测试排行榜变化
log ""
log "============================================"
log "  6. player2 解出第二题"
log "============================================"
CHAL2_ID="${CHAL_IDS[1]}"
CHAL2_FLAG="${CHAL_FLAGS[1]}"
SUBMIT2_PATH="/api/v1/competitions/$COMP_ID/challenges/$CHAL2_ID/submit"
resp=$(post_json "$CTF_URL$SUBMIT2_PATH" \
  "$(printf '{"flag":"%s"}' "$CHAL2_FLAG")" \
  "${USER_TOKENS[1]}")
echo "响应: $resp"
sleep 2

# 再次查看排行榜
log ""
log "============================================"
log "  7. 最终排行榜"
log "============================================"
get_json "$CTF_URL/api/v1/competitions/$COMP_ID/leaderboard" "${USER_TOKENS[0]}" | python3 -m json.tool

log ""
log "============================================"
log "  测试完成！"
log "============================================"
