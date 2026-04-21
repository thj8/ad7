#!/bin/bash
# quick-seed.sh — 快速生成测试数据（直接操作数据库）
# 生成少量测试数据用于验证一二三血和排行榜

DB_HOST="192.168.5.44"
DB_USER="root"
DB_PASS="asfdsfedarjeiowvgfsd"
DB_NAME="ctf"

mysql_cmd() {
  mysql -h "$DB_HOST" -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" -sNe "$1" 2>/dev/null
}

echo "=== 清理现有数据 ==="
mysql_cmd "DELETE FROM topthree_records"
mysql_cmd "DELETE FROM submissions"
mysql_cmd "DELETE FROM competition_challenges"
mysql_cmd "DELETE FROM challenges"
mysql_cmd "DELETE FROM competitions"
mysql_cmd "DELETE FROM team_members"
mysql_cmd "DELETE FROM users"
mysql_cmd "DELETE FROM teams"

echo "=== 创建 admin 用户（role=admin）==="
ADMIN_RES_ID=$(openssl rand -hex 16)
ADMIN_PASS_HASH='$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhW' # password123
mysql_cmd "INSERT INTO users (res_id, username, password_hash, role, created_at, updated_at, is_deleted) VALUES ('$ADMIN_RES_ID', 'admin', '$ADMIN_PASS_HASH', 'admin', NOW(), NOW(), 0)"
echo "  admin user created: $ADMIN_RES_ID"

echo "=== 创建测试用户（3个用于一二三血）==="
USER_RES_IDS=()
for i in 1 2 3 4; do
  USER_RES_ID=$(openssl rand -hex 16)
  USERNAME="player${i}"
  mysql_cmd "INSERT INTO users (res_id, username, password_hash, role, created_at, updated_at, is_deleted) VALUES ('$USER_RES_ID', '$USERNAME', '$ADMIN_PASS_HASH', 'member', NOW(), NOW(), 0)"
  USER_RES_IDS+=("$USER_RES_ID")
  echo "  player${i} created: $USER_RES_ID"
done

echo "=== 创建比赛 ==="
COMP_RES_ID=$(openssl rand -hex 16)
mysql_cmd "INSERT INTO competitions (res_id, title, description, start_time, end_time, is_active, created_at, updated_at, is_deleted) VALUES ('$COMP_RES_ID', '测试比赛', '用于验证一二三血和排行榜', '2026-01-01 00:00:00', '2026-12-31 23:59:59', 1, NOW(), NOW(), 0)"
echo "  competition created: $COMP_RES_ID"

echo "=== 创建2道题目 ==="
CHAL_RES_IDS=()
CHAL_FLAGS=()
for i in 1 2; do
  CHAL_RES_ID=$(openssl rand -hex 16)
  CHAL_TITLE="Web 题 ${i}"
  CHAL_SCORE=$((100 + i * 100))
  CHAL_FLAG="flag{test_challenge_${i}_$(openssl rand -hex 4)}"
  mysql_cmd "INSERT INTO challenges (res_id, title, description, category, score, flag, is_enabled, created_at, updated_at, is_deleted) VALUES ('$CHAL_RES_ID', '$CHAL_TITLE', '这是一道测试题目', 'web', $CHAL_SCORE, '$CHAL_FLAG', 1, NOW(), NOW(), 0)"
  CHAL_RES_IDS+=("$CHAL_RES_ID")
  CHAL_FLAGS+=("$CHAL_FLAG")
  echo "  challenge $i created: $CHAL_RES_ID (${CHAL_SCORE}分, flag=$CHAL_FLAG)"
done

echo "=== 将题目添加到比赛 ==="
for CHAL_RES_ID in "${CHAL_RES_IDS[@]}"; do
  CC_RES_ID=$(openssl rand -hex 16)
  mysql_cmd "INSERT INTO competition_challenges (res_id, competition_id, challenge_id) VALUES ('$CC_RES_ID', '$COMP_RES_ID', '$CHAL_RES_ID')"
done
echo "  done"

echo ""
echo "============================================"
echo "  测试数据生成完成！"
echo "============================================"
echo "  比赛 ID: $COMP_RES_ID"
echo "  题目 1 ID: ${CHAL_RES_IDS[0]} (flag: ${CHAL_FLAGS[0]})"
echo "  题目 2 ID: ${CHAL_RES_IDS[1]} (flag: ${CHAL_FLAGS[1]})"
echo "  用户: player1, player2, player3, player4"
echo "  管理员: admin / password123"
echo ""
echo "  下一步：启动服务器后，用不同用户提交 flag 来测试一二三血"
echo "============================================"
