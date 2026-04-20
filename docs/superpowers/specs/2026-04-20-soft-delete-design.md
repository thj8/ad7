# 软删除改造设计文档

**日期**: 2026-04-20
**状态**: 待审核

## 一、需求概述

### 1.1 原始需求

1. **所有关联都用 res_id，不允许用 id**
2. **项目中还有硬删除，全部改为软删除**

### 1.2 探索结果

- **需求 1（res_id 关联）**: ✅ 已确认无需修改 - 代码库中所有关联已正确使用 res_id
- **需求 2（硬删除改软删除）**: 需要修复 2 处硬删除，都在 `competition_challenges` 关联表

---

## 二、表结构设计

### 2.1 `competition_challenges` 表变更

#### 原有结构
```sql
CREATE TABLE IF NOT EXISTS competition_challenges (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32) NOT NULL UNIQUE,
    competition_id VARCHAR(32) NOT NULL,
    challenge_id   VARCHAR(32) NOT NULL,
    is_deleted     TINYINT(1)  NOT NULL DEFAULT 0,
    created_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_comp_chal (competition_id, challenge_id)
);
```

#### 变更后结构
```sql
CREATE TABLE IF NOT EXISTS competition_challenges (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32) NOT NULL UNIQUE,
    competition_id VARCHAR(32) NOT NULL,
    challenge_id   VARCHAR(32) NOT NULL,
    is_deleted     TINYINT(1)  NOT NULL DEFAULT 0,
    deleted_at     DATETIME     DEFAULT NULL,  -- 新增字段
    created_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_comp_chal_del (competition_id, challenge_id, deleted_at)  -- 修改索引
);
```

### 2.2 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `is_deleted` | TINYINT(1) | 0 | 软删除标记（保留，向后兼容） |
| `deleted_at` | DATETIME | NULL | 删除时间（NULL 表示未删除） |

### 2.3 索引说明

- **唯一索引**：`(competition_id, challenge_id, deleted_at)`
  - 允许同一比赛-题目对有多条已删除记录（删除时间不同）
  - 只允许一条未删除记录（`deleted_at` = NULL）

---

## 三、代码变更

### 3.1 修改文件清单

| 文件路径 | 变更类型 | 说明 |
|---------|---------|------|
| `internal/store/mysql.go` | 修改 | 4 个函数变更 |
| `sql/schema.sql` | 修改 | 更新表结构定义 |
| `sql/migrations/...` | 新增 | 数据库迁移脚本（可选） |

### 3.2 `DeleteCompetition` 函数

**位置**：`internal/store/mysql.go:258-267`

**变更前**：
```go
func (s *Store) DeleteCompetition(ctx context.Context, resID string) error {
    // 先清理比赛与题目的关联记录（硬删除）
    if _, err := s.db.ExecContext(ctx, `DELETE FROM competition_challenges WHERE competition_id = ?`, resID); err != nil {
        return fmt.Errorf("delete competition challenges for %s: %w", resID, err)
    }
    // 软删除比赛本身
    _, err := s.db.ExecContext(ctx, `UPDATE competitions SET is_deleted=1 WHERE res_id = ?`, resID)
    return fmt.Errorf("delete competition %s: %w", resID, err)
}
```

**变更后**：
```go
func (s *Store) DeleteCompetition(ctx context.Context, resID string) error {
    // 软删除比赛与题目的关联记录
    if _, err := s.db.ExecContext(ctx,
        `UPDATE competition_challenges
         SET is_deleted = 1, deleted_at = NOW()
         WHERE competition_id = ? AND deleted_at IS NULL`, resID); err != nil {
        return fmt.Errorf("soft delete competition challenges for %s: %w", resID, err)
    }
    // 软删除比赛本身
    _, err := s.db.ExecContext(ctx, `UPDATE competitions SET is_deleted=1 WHERE res_id = ?`, resID)
    return fmt.Errorf("delete competition %s: %w", resID, err)
}
```

### 3.3 `RemoveChallenge` 函数

**位置**：`internal/store/mysql.go:277-283`

**变更前**：
```go
func (s *Store) RemoveChallenge(ctx context.Context, compID, chalID string) error {
    _, err := s.db.ExecContext(ctx,
        `DELETE FROM competition_challenges WHERE competition_id = ? AND challenge_id = ?`,
        compID, chalID)
    return fmt.Errorf("error: %w", err)
}
```

**变更后**：
```go
func (s *Store) RemoveChallenge(ctx context.Context, compID, chalID string) error {
    _, err := s.db.ExecContext(ctx,
        `UPDATE competition_challenges
         SET is_deleted = 1, deleted_at = NOW()
         WHERE competition_id = ? AND challenge_id = ? AND deleted_at IS NULL`,
        compID, chalID)
    return fmt.Errorf("soft remove challenge: %w", err)
}
```

### 3.4 `ListCompChallenges` 函数

**位置**：`internal/store/mysql.go:285-306`

**变更前**：
```go
func (s *Store) ListCompChallenges(ctx context.Context, compID string) ([]model.Challenge, error) {
    rows, err := s.db.QueryContext(ctx,
        `SELECT c.res_id, c.title, c.category, c.description, c.score, c.is_enabled, c.created_at, c.updated_at
         FROM challenges c
         JOIN competition_challenges cc ON cc.challenge_id = c.res_id
         WHERE cc.competition_id = ? AND c.is_enabled = 1 AND c.is_deleted = 0`, compID)
    // ...
}
```

**变更后**：
```go
func (s *Store) ListCompChallenges(ctx context.Context, compID string) ([]model.Challenge, error) {
    rows, err := s.db.QueryContext(ctx,
        `SELECT c.res_id, c.title, c.category, c.description, c.score, c.is_enabled, c.created_at, c.updated_at
         FROM challenges c
         JOIN competition_challenges cc ON cc.challenge_id = c.res_id
         WHERE cc.competition_id = ?
           AND cc.is_deleted = 0
           AND cc.deleted_at IS NULL
           AND c.is_enabled = 1
           AND c.is_deleted = 0`, compID)
    // ...
}
```

### 3.5 `AddChallenge` 函数

**位置**：`internal/store/mysql.go:269-275`

**变更**：无需修改代码，但行为变化

**说明**：由于唯一索引包含 `deleted_at`，即使之前有软删除记录，也可以成功插入新记录。

---

## 四、查询过滤规则

所有 `competition_challenges` 表的查询都必须添加以下过滤条件：

```sql
WHERE cc.is_deleted = 0 AND cc.deleted_at IS NULL
```

---

## 五、数据库迁移

### 5.1 迁移脚本（可选）

```sql
-- 1. 添加 deleted_at 字段
ALTER TABLE competition_challenges
ADD COLUMN deleted_at DATETIME DEFAULT NULL AFTER is_deleted;

-- 2. 将已软删除的记录的 deleted_at 设为 updated_at
UPDATE competition_challenges
SET deleted_at = updated_at
WHERE is_deleted = 1;

-- 3. 删除旧索引
DROP INDEX idx_comp_chal ON competition_challenges;

-- 4. 创建新索引
CREATE UNIQUE INDEX idx_comp_chal_del ON competition_challenges
(competition_id, challenge_id, deleted_at);
```

---

## 六、测试说明

### 6.1 需要测试的场景

1. **删除比赛**：验证 `competition_challenges` 记录被软删除而非硬删除
2. **移除题目**：验证 `RemoveChallenge` 软删除关联记录
3. **查询题目列表**：验证软删除的关联不会出现在结果中
4. **重新添加题目**：验证同一题目可以被重新添加到比赛中

### 6.2 测试文件

- `internal/integration/` 中的集成测试需要更新
- `cmd/seed/` 和 `internal/testutil/` 中的硬删除可以保留（测试清理用）

---

## 七、向后兼容性

- ✅ 保留 `is_deleted` 字段，现有代码不会破坏
- ✅ `deleted_at` 字段有默认值 NULL，现有查询不会报错
- ⚠️ 需要确保所有新查询都包含 `deleted_at IS NULL` 过滤

---

## 八、总结

| 项目 | 状态 |
|------|------|
| 需求 1（res_id 关联）| ✅ 无需修改 |
| 需求 2（硬删除改软删除）| 🔄 待实施 |
| 涉及表 | `competition_challenges` |
| 涉及函数 | 4 个 store 函数 |
| 数据库迁移 | 需要（可选） |
