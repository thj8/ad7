# 软删除改造 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `competition_challenges` 表的两处硬删除改为软删除，新增 `deleted_at` 字段并更新查询过滤。

**Architecture:** 保留 `is_deleted` 字段（向后兼容），新增 `deleted_at` 字段记录删除时间，修改唯一索引包含 `deleted_at`，更新 store 层函数。

**Tech Stack:** Go, MySQL

---

## 文件映射

| 文件路径 | 变更类型 | 责任 |
|---------|---------|------|
| `sql/schema.sql` | 修改 | 更新 `competition_challenges` 表结构定义 |
| `internal/store/mysql.go` | 修改 | 更新 4 个 store 函数 |

---

## 任务分解

### Task 1: 更新 SQL Schema

**Files:**
- Modify: `sql/schema.sql`

- [ ] **Step 1: 读取当前 schema**

先读取现有 schema 文件内容。

- [ ] **Step 2: 更新 `competition_challenges` 表定义**

找到 `CREATE TABLE IF NOT EXISTS competition_challenges` 语句，做以下修改：

1. 在 `is_deleted` 字段后添加 `deleted_at` 字段：
   ```sql
   deleted_at     DATETIME     DEFAULT NULL,
   ```

2. 修改唯一索引：
   ```sql
   UNIQUE INDEX idx_comp_chal_del (competition_id, challenge_id, deleted_at)
   ```

完整表结构应为：
```sql
CREATE TABLE IF NOT EXISTS competition_challenges (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32) NOT NULL UNIQUE,
    competition_id VARCHAR(32) NOT NULL,
    challenge_id   VARCHAR(32) NOT NULL,
    is_deleted     TINYINT(1)  NOT NULL DEFAULT 0,
    deleted_at     DATETIME     DEFAULT NULL,
    created_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_comp_chal_del (competition_id, challenge_id, deleted_at)
);
```

- [ ] **Step 3: 保存文件并验证**

保存 `sql/schema.sql`。

- [ ] **Step 4: Commit**

```bash
git add sql/schema.sql
git commit -m "refactor: 更新 competition_challenges schema 支持软删除"
```

---

### Task 2: 修改 `DeleteCompetition` 函数

**Files:**
- Modify: `internal/store/mysql.go:258-267`

- [ ] **Step 1: 读取当前代码**

确认 `DeleteCompetition` 函数位置和内容。

- [ ] **Step 2: 修改函数实现**

将硬删除 `DELETE FROM competition_challenges` 改为软删除 `UPDATE`：

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

- [ ] **Step 3: 运行编译验证**

```bash
go build ./internal/store/...
```

Expected: 编译成功，无错误。

- [ ] **Step 4: Commit**

```bash
git add internal/store/mysql.go
git commit -m "refactor: DeleteCompetition 使用软删除"
```

---

### Task 3: 修改 `RemoveChallenge` 函数

**Files:**
- Modify: `internal/store/mysql.go:277-283`

- [ ] **Step 1: 读取当前代码**

确认 `RemoveChallenge` 函数位置和内容。

- [ ] **Step 2: 修改函数实现**

将硬删除 `DELETE FROM` 改为软删除 `UPDATE`：

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

- [ ] **Step 3: 运行编译验证**

```bash
go build ./internal/store/...
```

Expected: 编译成功，无错误。

- [ ] **Step 4: Commit**

```bash
git add internal/store/mysql.go
git commit -m "refactor: RemoveChallenge 使用软删除"
```

---

### Task 4: 修改 `ListCompChallenges` 函数

**Files:**
- Modify: `internal/store/mysql.go:285-306`

- [ ] **Step 1: 读取当前代码**

确认 `ListCompChallenges` 函数位置和内容。

- [ ] **Step 2: 修改查询语句**

添加 `cc.is_deleted = 0 AND cc.deleted_at IS NULL` 过滤条件：

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
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var cs []model.Challenge
    for rows.Next() {
        var c model.Challenge
        if err := rows.Scan(&c.ResID, &c.Title, &c.Category, &c.Description, &c.Score, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
            return nil, err
        }
        cs = append(cs, c)
    }
    return cs, rows.Err()
}
```

- [ ] **Step 3: 运行编译验证**

```bash
go build ./internal/store/...
```

Expected: 编译成功，无错误。

- [ ] **Step 4: Commit**

```bash
git add internal/store/mysql.go
git commit -m "refactor: ListCompChallenges 过滤软删除记录"
```

---

### Task 5: 运行完整编译和测试

**Files:**
- 无文件修改

- [ ] **Step 1: 运行完整编译**

```bash
go build ./...
```

Expected: 编译成功，无错误。

- [ ] **Step 2: 运行所有测试（无 MySQL 时跳过集成测试）**

```bash
go test ./internal/store/... -v -short
```

Expected: 所有单元测试通过。

---

## Self-Review

### 1. Spec Coverage

| Spec 需求 | 实现任务 |
|----------|---------|
| 新增 `deleted_at` 字段 | Task 1 |
| 修改唯一索引包含 `deleted_at` | Task 1 |
| `DeleteCompetition` 软删除 | Task 2 |
| `RemoveChallenge` 软删除 | Task 3 |
| `ListCompChallenges` 过滤 | Task 4 |
| 保留 `is_deleted` 向后兼容 | Task 1-4 |

### 2. Placeholder Scan

✅ 无 TBD/TODO，所有步骤包含完整代码和命令

### 3. Type Consistency

✅ 所有函数签名、字段名保持一致

---

## Plan Complete

计划已保存至 `docs/superpowers/plans/2026-04-20-soft-delete-plan.md`。

**两种执行选项：**

**1. Subagent-Driven（推荐）** - 每个任务使用独立子代理执行，任务间进行审查，快速迭代

**2. Inline Execution** - 在当前会话中使用 executing-plans 执行，带检查点的批量执行

选择哪种方式？
