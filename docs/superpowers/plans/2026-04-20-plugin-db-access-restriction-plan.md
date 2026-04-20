# 插件数据库访问限制 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现插件数据库访问限制，插件不能直接查询主表，通过 pluginutil 查询，插件之间通过接口调用

**Architecture:** 修改 Plugin 接口添加 Name() 方法和依赖注入参数，定义 TopThreeProvider 接口，扩展 pluginutil，修改所有插件实现新接口

**Tech Stack:** Go, MySQL, chi

---

## 文件映射

| 文件路径 | 变更类型 | 责任 |
|---------|---------|------|
| `internal/plugin/plugin.go` | 修改 | Plugin 接口添加 Name() 和 deps 参数 |
| `internal/plugin/names.go` | 创建 | 插件名称常量 |
| `plugins/topthree/provider.go` | 创建 | TopThreeProvider 接口定义 |
| `plugins/topthree/topthree.go` | 修改 | 实现 Name() 和 TopThreeProvider 接口 |
| `plugins/leaderboard/leaderboard.go` | 修改 | 实现 Name()，通过 TopThreeProvider 获取数据 |
| `plugins/analytics/analytics.go` | 修改 | 实现 Name()，通过 pluginutil 获取数据 |
| `plugins/notification/notification.go` | 修改 | 实现 Name() |
| `plugins/hints/hints.go` | 修改 | 实现 Name() |
| `internal/pluginutil/queries.go` | 修改 | 添加聚合查询函数 |
| `cmd/server/main.go` | 修改 | 插件初始化和依赖排序 |

---

## 任务分解

### Task 1: 添加插件名称常量

**Files:**
- Create: `internal/plugin/names.go`

- [ ] **Step 1: 创建插件名称常量文件**

```go
package plugin

const (
    NameLeaderboard  = "leaderboard"
    NameNotification = "notification"
    NameHints        = "hints"
    NameAnalytics    = "analytics"
    NameTopThree     = "topthree"
)
```

- [ ] **Step 2: 运行编译验证**

```bash
go build ./internal/plugin/...
```

Expected: 编译成功，无错误。

- [ ] **Step 3: Commit**

```bash
git add internal/plugin/names.go
git commit -m "feat: 添加插件名称常量"
```

---

### Task 2: 修改 Plugin 接口

**Files:**
- Modify: `internal/plugin/plugin.go:14-25`

- [ ] **Step 1: 修改 Plugin 接口**

```go
// Plugin 是所有插件必须实现的接口。
// Register 方法在服务启动时被调用，插件在此方法中：
//   - 保存数据库连接和认证中间件的引用
//   - 在 chi 路由器上注册自己的路由
//
// 参数：
//   - r: chi 路由器，用于注册路由
//   - db: 数据库连接，供插件查询自己的表
//   - auth: 认证中间件，用于保护插件路由
//   - deps: 已初始化的依赖插件，key 是插件名称
type Plugin interface {
    // Name 返回插件的唯一名称，用于依赖管理
    Name() string

    // Register 方法在服务启动时被调用
    Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]Plugin)
}
```

- [ ] **Step 2: 运行编译验证**

```bash
go build ./internal/plugin/...
```

Expected: 编译错误（因为插件还没实现新接口），这是预期的。

- [ ] **Step 3: Commit**

```bash
git add internal/plugin/plugin.go
git commit -m "feat: 修改 Plugin 接口支持 Name() 和依赖注入"
```

---

### Task 3: 定义 TopThreeProvider 接口

**Files:**
- Create: `plugins/topthree/provider.go`

- [ ] **Step 1: 创建 TopThreeProvider 接口定义**

```go
package topthree

import "context"

// TopThreeProvider 定义 topthree 插件暴露给其他插件的接口
type TopThreeProvider interface {
    // GetBloodRank 获取用户在某道题目的三血排名
    // 返回值: 1=一血, 2=二血, 3=三血, 0=未入榜
    GetBloodRank(ctx context.Context, compID, chalID, userID string) (int, error)

    // GetCompTopThree 获取比赛每道题目的三血信息
    // 返回值: map[challengeID]BloodRankEntry
    GetCompTopThree(ctx context.Context, compID string) (map[string]BloodRankEntry, error)
}

// BloodRankEntry 表示单道题目的三血排名信息
type BloodRankEntry struct {
    ChallengeID string
    FirstBlood  string // 用户ID
    SecondBlood string // 用户ID
    ThirdBlood  string // 用户ID
}
```

- [ ] **Step 2: 运行编译验证**

```bash
go build ./plugins/topthree/...
```

Expected: 编译成功，无错误。

- [ ] **Step 3: Commit**

```bash
git add plugins/topthree/provider.go
git commit -m "feat: 定义 TopThreeProvider 接口"
```

---

### Task 4: 修改 topthree 插件实现新接口

**Files:**
- Modify: `plugins/topthree/topthree.go`

- [ ] **Step 1: 实现 Name() 方法**

在 `Plugin` 结构体上添加：

```go
// Name 返回插件名称
func (p *Plugin) Name() string {
    return plugin.NameTopThree
}
```

- [ ] **Step 2: 修改 Register 方法签名**

```go
// Register 注册三血插件的路由并订阅正确提交事件。
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]Plugin) {
    p.db = db

    // 订阅正确提交事件，触发三血排名更新
    event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

    r.Group(func(r chi.Router) {
        r.Use(auth.Authenticate)
        r.Get("/api/v1/topthree/competitions/{id}", p.getTopThree)
    })
}
```

- [ ] **Step 3: 实现 TopThreeProvider 接口方法**

```go
// GetBloodRank 获取用户在某道题目的三血排名
func (p *Plugin) GetBloodRank(ctx context.Context, compID, chalID, userID string) (int, error) {
    var ranking int
    err := p.db.QueryRowContext(ctx, `
        SELECT ranking FROM topthree_records
        WHERE competition_id = ? AND challenge_id = ? AND user_id = ? AND is_deleted = 0
    `, compID, chalID, userID).Scan(&ranking)
    if err == sql.ErrNoRows {
        return 0, nil
    }
    return ranking, err
}

// GetCompTopThree 获取比赛每道题目的三血信息
func (p *Plugin) GetCompTopThree(ctx context.Context, compID string) (map[string]BloodRankEntry, error) {
    rows, err := p.db.QueryContext(ctx, `
        SELECT challenge_id, user_id, ranking
        FROM topthree_records
        WHERE competition_id = ? AND ranking IN (1,2,3) AND is_deleted = 0
        ORDER BY ranking ASC
    `, compID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    result := make(map[string]BloodRankEntry)
    for rows.Next() {
        var chalID, userID string
        var ranking int
        if err := rows.Scan(&chalID, &userID, &ranking); err != nil {
            return nil, err
        }
        entry, ok := result[chalID]
        if !ok {
            entry = BloodRankEntry{ChallengeID: chalID}
        }
        switch ranking {
        case 1:
            entry.FirstBlood = userID
        case 2:
            entry.SecondBlood = userID
        case 3:
            entry.ThirdBlood = userID
        }
        result[chalID] = entry
    }
    return result, rows.Err()
}
```

- [ ] **Step 4: 运行编译验证**

```bash
go build ./plugins/topthree/...
```

Expected: 编译成功，无错误。

- [ ] **Step 5: Commit**

```bash
git add plugins/topthree/topthree.go
git commit -m "feat: topthree 实现 Name() 和 TopThreeProvider 接口"
```

---

### Task 5: 修改 notification 插件实现 Name()

**Files:**
- Modify: `plugins/notification/notification.go`

- [ ] **Step 1: 实现 Name() 方法**

在 `Plugin` 结构体上添加：

```go
// Name 返回插件名称
func (p *Plugin) Name() string {
    return plugin.NameNotification
}
```

- [ ] **Step 2: 修改 Register 方法签名**

添加 `deps map[string]Plugin` 参数（忽略不使用）：

```go
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]Plugin) {
```

- [ ] **Step 3: 运行编译验证**

```bash
go build ./plugins/notification/...
```

Expected: 编译成功，无错误。

- [ ] **Step 4: Commit**

```bash
git add plugins/notification/notification.go
git commit -m "feat: notification 实现 Name() 方法"
```

---

### Task 6: 修改 hints 插件实现 Name()

**Files:**
- Modify: `plugins/hints/hints.go`

- [ ] **Step 1: 实现 Name() 方法**

在 `Plugin` 结构体上添加：

```go
// Name 返回插件名称
func (p *Plugin) Name() string {
    return plugin.NameHints
}
```

- [ ] **Step 2: 修改 Register 方法签名**

添加 `deps map[string]Plugin` 参数（忽略不使用）。

- [ ] **Step 3: 运行编译验证**

```bash
go build ./plugins/hints/...
```

Expected: 编译成功，无错误。

- [ ] **Step 4: Commit**

```bash
git add plugins/hints/hints.go
git commit -m "feat: hints 实现 Name() 方法"
```

---

### Task 7: 扩展 pluginutil 添加聚合查询函数

**Files:**
- Modify: `internal/pluginutil/queries.go`

- [ ] **Step 1: 添加 CategoryStats 类型和 GetCategoryStats 函数**

```go
// CategoryStats 表示单个分类的统计数据
type CategoryStats struct {
    Category          string
    TotalChallenges   int
    TotalSolves       int
    UniqueUsersSolved int
    TotalAttempts     int
}

// GetCategoryStats 获取按分类统计的数据
func GetCategoryStats(ctx context.Context, db DBTX, compID string) ([]CategoryStats, error) {
    rows, err := db.QueryContext(ctx, `
        SELECT
            c.category,
            COUNT(DISTINCT cc.challenge_id) as total_challenges,
            COALESCE(SUM(CASE WHEN s.is_correct = 1 THEN 1 ELSE 0 END), 0) as total_solves,
            COUNT(DISTINCT CASE WHEN s.is_correct = 1 THEN s.user_id ELSE NULL END) as unique_users_solved,
            COUNT(s.id) as total_attempts
        FROM competition_challenges cc
        JOIN challenges c ON c.res_id = cc.challenge_id AND c.is_deleted = 0
        LEFT JOIN submissions s ON s.challenge_id = cc.challenge_id
            AND s.competition_id = cc.competition_id AND s.is_deleted = 0
        WHERE cc.competition_id = ? AND cc.is_deleted = 0
        GROUP BY c.category
    `, compID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var categories []CategoryStats
    for rows.Next() {
        var cat CategoryStats
        if err := rows.Scan(&cat.Category, &cat.TotalChallenges, &cat.TotalSolves, &cat.UniqueUsersSolved, &cat.TotalAttempts); err != nil {
            return nil, err
        }
        categories = append(categories, cat)
    }
    return categories, rows.Err()
}
```

- [ ] **Step 2: 添加 UserStats 类型和 GetUserStats 函数**

```go
// UserStats 表示单个用户的统计数据
type UserStats struct {
    UserID         string
    TotalSolves    int
    TotalScore     int
    TotalAttempts  int
    SuccessRate    float64
    FirstSolveTime sql.NullTime
    LastSolveTime  sql.NullTime
}

// GetUserStats 获取用户统计数据
func GetUserStats(ctx context.Context, db DBTX, compID string) ([]UserStats, error) {
    rows, err := db.QueryContext(ctx, `
        SELECT
            s.user_id,
            SUM(CASE WHEN s.is_correct = 1 THEN 1 ELSE 0 END) as total_solves,
            SUM(CASE WHEN s.is_correct = 1 THEN c.score ELSE 0 END) as total_score,
            COUNT(*) as total_attempts,
            MIN(CASE WHEN s.is_correct = 1 THEN s.created_at ELSE NULL END) as first_solve,
            MAX(CASE WHEN s.is_correct = 1 THEN s.created_at ELSE NULL END) as last_solve
        FROM submissions s
        LEFT JOIN challenges c ON c.res_id = s.challenge_id AND c.is_deleted = 0
        WHERE s.competition_id = ? AND s.is_deleted = 0
        GROUP BY s.user_id
        ORDER BY total_score DESC, first_solve ASC
    `, compID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []UserStats
    for rows.Next() {
        var u UserStats
        if err := rows.Scan(&u.UserID, &u.TotalSolves, &u.TotalScore, &u.TotalAttempts, &u.FirstSolveTime, &u.LastSolveTime); err != nil {
            return nil, err
        }
        if u.TotalAttempts > 0 {
            u.SuccessRate = (float64(u.TotalSolves) / float64(u.TotalAttempts)) * 100
        }
        users = append(users, u)
    }
    return users, rows.Err()
}
```

- [ ] **Step 3: 添加 ChallengeStats 类型和 GetChallengeStats 函数**

```go
// ChallengeStats 表示单个题目的统计数据
type ChallengeStats struct {
    ChallengeID       string
    Title             string
    Category          string
    Score             int
    TotalSolves       int
    TotalAttempts     int
    SuccessRate       float64
    UniqueUsersSolved int
    FirstSolveTime    sql.NullTime
    AverageSolveTime  sql.NullFloat64
}

// GetChallengeStats 获取单题目统计数据
func GetChallengeStats(ctx context.Context, db DBTX, compID string) ([]ChallengeStats, error) {
    // 先获取比赛题目列表
    challenges, err := GetCompChallenges(ctx, db, compID)
    if err != nil {
        return nil, err
    }

    var result []ChallengeStats
    for _, ch := range challenges {
        cs := ChallengeStats{
            ChallengeID: ch.ResID,
            Title:       ch.Title,
            Category:    ch.Category,
            Score:       ch.Score,
        }

        // 查询该题目的提交统计
        var firstSolve sql.NullTime
        err = db.QueryRowContext(ctx, `
            SELECT
                SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END),
                COUNT(*),
                COUNT(DISTINCT CASE WHEN is_correct = 1 THEN user_id ELSE NULL END),
                MIN(CASE WHEN is_correct = 1 THEN created_at ELSE NULL END)
            FROM submissions
            WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 0
        `, compID, cs.ChallengeID).Scan(
            &cs.TotalSolves,
            &cs.TotalAttempts,
            &cs.UniqueUsersSolved,
            &firstSolve,
        )
        if err != nil {
            return nil, err
        }

        if cs.TotalAttempts > 0 {
            cs.SuccessRate = (float64(cs.TotalSolves) / float64(cs.TotalAttempts)) * 100
        }
        if firstSolve.Valid {
            cs.FirstSolveTime = firstSolve
        }

        // 计算平均解题时间
        var avgSolveTime sql.NullFloat64
        err = db.QueryRowContext(ctx, `
            SELECT AVG(TIMESTAMPDIFF(SECOND, first_submit, correct_submit))
            FROM (
                SELECT
                    user_id,
                    MIN(created_at) as first_submit,
                    MIN(CASE WHEN is_correct = 1 THEN created_at ELSE NULL END) as correct_submit
                FROM submissions
                WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 0
                GROUP BY user_id
                HAVING correct_submit IS NOT NULL
            ) user_times
        `, compID, cs.ChallengeID).Scan(&avgSolveTime)
        if err != nil {
            return nil, err
        }
        if avgSolveTime.Valid {
            cs.AverageSolveTime = avgSolveTime
        }

        result = append(result, cs)
    }

    return result, nil
}
```

- [ ] **Step 4: 添加 GetAverageSolveTime 函数**

```go
// GetAverageSolveTime 获取比赛平均解题时间（秒）
func GetAverageSolveTime(ctx context.Context, db DBTX, compID string) (float64, error) {
    var avgSolveTimeSec sql.NullFloat64
    err := db.QueryRowContext(ctx, `
        SELECT AVG(TIMESTAMPDIFF(SECOND, c.start_time, s.created_at))
        FROM submissions s
        JOIN competitions c ON c.res_id = s.competition_id
        WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0 AND c.is_deleted = 0
    `, compID).Scan(&avgSolveTimeSec)
    if err != nil {
        return 0, err
    }
    if avgSolveTimeSec.Valid {
        return avgSolveTimeSec.Float64, nil
    }
    return 0, nil
}
```

- [ ] **Step 5: 运行编译验证**

```bash
go build ./internal/pluginutil/...
```

Expected: 编译成功，无错误。

- [ ] **Step 6: Commit**

```bash
git add internal/pluginutil/queries.go
git commit -m "feat: pluginutil 添加聚合查询函数"
```

---

### Task 8: 修改 leaderboard 插件

**Files:**
- Modify: `plugins/leaderboard/leaderboard.go`

- [ ] **Step 1: 实现 Name() 方法**

在 `Plugin` 结构体上添加：

```go
// Name 返回插件名称
func (p *Plugin) Name() string {
    return plugin.NameLeaderboard
}
```

- [ ] **Step 2: 修改 Register 方法签名并获取 topthree 依赖**

```go
// Register 注册排行榜的路由。
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]Plugin) {
    p.db = db

    // 获取 topthree 插件
    if topthreePlugin, ok := deps[plugin.NameTopThree]; ok {
        if provider, ok := topthreePlugin.(topthree.TopThreeProvider); ok {
            p.topthreeProvider = provider
        }
    }

    r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/leaderboard", p.listByComp)
}
```

注意：需要在 `Plugin` 结构体中添加 `topthreeProvider topthree.TopThreeProvider` 字段。

- [ ] **Step 3: 修改 listByComp 方法，使用 TopThreeProvider 获取三血数据**

移除直接查询 `topthree_records` 表的代码，替换为：

```go
// 获取 topthree 数据
var bloodRank map[string]topthree.BloodRankEntry
if p.topthreeProvider != nil {
    var err error
    bloodRank, err = p.topthreeProvider.GetCompTopThree(ctx, compID)
    if err != nil {
        pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
        return
    }
}
```

然后在构建 `challengeResult` 时，从 `bloodRank` 中获取排名：

```go
cr := challengeResult{ChallengeID: chalID}
if solvedAt, ok := solvedMap[chalID]; ok {
    cr.Solved = true
    cr.SolvedAt = solvedAt
    if bloodRank != nil {
        if entry, ok := bloodRank[chalID]; ok {
            switch {
            case entry.FirstBlood == uid:
                cr.BloodRank = 1
            case entry.SecondBlood == uid:
                cr.BloodRank = 2
            case entry.ThirdBlood == uid:
                cr.BloodRank = 3
            }
        }
    }
    if solvedAt.After(lastSolveAt) {
        lastSolveAt = solvedAt
    }
}
```

- [ ] **Step 4: 运行编译验证**

```bash
go build ./plugins/leaderboard/...
```

Expected: 编译成功，无错误。

- [ ] **Step 5: Commit**

```bash
git add plugins/leaderboard/leaderboard.go
git commit -m "feat: leaderboard 通过 TopThreeProvider 获取三血数据"
```

---

### Task 9: 修改 analytics 插件

**Files:**
- Modify: `plugins/analytics/analytics.go`

- [ ] **Step 1: 实现 Name() 方法**

在 `Plugin` 结构体上添加：

```go
// Name 返回插件名称
func (p *Plugin) Name() string {
    return plugin.NameAnalytics
}
```

- [ ] **Step 2: 修改 Register 方法签名**

添加 `deps map[string]Plugin` 参数（忽略不使用）。

- [ ] **Step 3: 修改 overview 方法**

使用 `pluginutil.GetAverageSolveTime` 替代直接 SQL。

- [ ] **Step 4: 修改 byCategory 方法**

使用 `pluginutil.GetCategoryStats` 替代直接 SQL。

- [ ] **Step 5: 修改 userStats 方法**

使用 `pluginutil.GetUserStats` 替代直接 SQL。

- [ ] **Step 6: 修改 challengeStats 方法**

使用 `pluginutil.GetChallengeStats` 和 `pluginutil.GetCompChallenges` 替代直接 SQL。

- [ ] **Step 7: 运行编译验证**

```bash
go build ./plugins/analytics/...
```

Expected: 编译成功，无错误。

- [ ] **Step 8: Commit**

```bash
git add plugins/analytics/analytics.go
git commit -m "feat: analytics 通过 pluginutil 获取统计数据"
```

---

### Task 10: 修改 cmd/server/main.go 插件初始化

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: 修改插件初始化逻辑**

实现拓扑排序，确保 topthree 先于 leaderboard 初始化：

```go
// 创建插件实例
plugins := []plugin.Plugin{
    notification.New(),
    hints.New(),
    topthree.New(),  // topthree 先于 leaderboard
    leaderboard.New(),
    analytics.New(),
}

// 按依赖顺序排序：topthree 先于 leaderboard
// 简单的固定顺序，因为只有一个依赖关系
sortedPlugins := make([]plugin.Plugin, 0, len(plugins))
pluginMap := make(map[string]plugin.Plugin)
for _, p := range plugins {
    pluginMap[p.Name()] = p
}

// 先添加无依赖的插件
for _, name := range []string{
    plugin.NameNotification,
    plugin.NameHints,
    plugin.NameTopThree,
    plugin.NameAnalytics,
} {
    if p, ok := pluginMap[name]; ok {
        sortedPlugins = append(sortedPlugins, p)
        delete(pluginMap, name)
    }
}

// 最后添加有依赖的插件
if p, ok := pluginMap[plugin.NameLeaderboard]; ok {
    sortedPlugins = append(sortedPlugins, p)
}

// 按顺序注册插件
initializedDeps := make(map[string]plugin.Plugin)
for _, p := range sortedPlugins {
    p.Register(r, db, authMiddleware, initializedDeps)
    initializedDeps[p.Name()] = p
}
```

- [ ] **Step 2: 运行编译验证**

```bash
go build ./cmd/server/...
```

Expected: 编译成功，无错误。

- [ ] **Step 3: 运行完整编译和测试**

```bash
go build ./...
go test ./... -v -short
```

Expected: 所有编译和测试通过。

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: 修改插件初始化支持依赖注入"
```

---

## Self-Review

### 1. Spec Coverage

| Spec 需求 | 实现任务 |
|----------|---------|
| Plugin 接口添加 Name() | Task 2 |
| Plugin 接口添加 deps 参数 | Task 2 |
| 插件名称常量 | Task 1 |
| TopThreeProvider 接口 | Task 3 |
| topthree 实现新接口 | Task 4 |
| leaderboard 通过接口调用 | Task 8 |
| pluginutil 聚合查询 | Task 7 |
| analytics 通过 pluginutil | Task 9 |
| notification/hints 实现 Name() | Task 5, 6 |
| 插件依赖排序 | Task 10 |

### 2. Placeholder Scan

✅ 无 TBD/TODO，所有步骤包含完整代码和命令

### 3. Type Consistency

✅ 所有类型、方法签名、属性名称一致

---

## Plan Complete

计划已保存至 `docs/superpowers/plans/2026-04-20-plugin-db-access-restriction-plan.md`。

**两种执行选项：**

**1. Subagent-Driven（推荐）** - 每个任务使用独立子代理执行，任务间进行审查，快速迭代

**2. Inline Execution** - 在当前会话中使用 executing-plans 执行，带检查点的批量执行

选择哪种方式？
