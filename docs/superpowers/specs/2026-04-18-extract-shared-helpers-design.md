# 重构：提取共享辅助函数 + Store 层统一

## 目标

消除代码库中的重复模式：插件间共享查询函数、HTTP 响应辅助、ID 校验；Store 层合并方法对。

## 范围

- 新增 `internal/pluginutil/` 共享包
- Store 层合并 3 对方法
- Handler 层跟随 Store 接口变化
- 所有插件改用 pluginutil
- **不改动** Service 层（Submit/SubmitInComp 保持独立）

## 约束

- 插件私有表（topthree_records、hints、notifications）不在 pluginutil 中访问
- 不改变公共 API（路由、请求/响应格式不变）

---

## 1. 新增 `internal/pluginutil/` 包

### 1.1 `queries.go` — 共享数据库查询

封装对**主程序表**（challenges、submissions、competition_challenges、competitions）的常用查询。

所有函数接收 `DBTX` 接口（`QueryContext` + `QueryRowContext`），方便测试时 mock：

```go
type DBTX interface {
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

**函数列表：**

| 函数签名 | 用途 | 当前调用方 |
|---------|------|-----------|
| `GetCompChallenges(ctx, db, compID) ([]ChallengeInfo, error)` | 获取比赛下题目信息（res_id, title, category, score） | leaderboard（提取 ID），analytics challengeStats |
| `GetFirstCorrectSubmissions(ctx, db, compID) ([]FirstSolve, error)` | 每用户每题最早正确提交 | leaderboard |
| `GetUserScores(ctx, db, compID) (map[string]int, error)` | 比赛内每用户总分 | leaderboard |
| `GetCompSubmitStats(ctx, db, compID) (total, correct int, err error)` | 比赛总提交数和正确数 | analytics overview |
| `GetCompDistinctUsers(ctx, db, compID) (int, error)` | 比赛内独立用户数 | analytics overview, byCategory |
| `GetCompChallengeCount(ctx, db, compID) (int, error)` | 比赛题目总数 | analytics overview |

**类型定义：**

```go
// ChallengeInfo 题目摘要信息（不含 Flag 和 description）
type ChallengeInfo struct {
    ResID    string
    Title    string
    Category string
    Score    int
}

// FirstSolve 用户在某题的最早正确提交
type FirstSolve struct {
    UserID      string
    ChallengeID string
    SolvedAt    time.Time
}
```

**注意**：`GetCompChallenges` 替代原来的 `GetCompChallengeIDs`。调用方只需 ID 时，从返回的 `[]ChallengeInfo` 中提取 `.ResID` 即可。leaderboard 原来只查 ID 的地方改为调用 `GetCompChallenges` 然后遍历取 ResID。

### 1.2 `http.go` — HTTP 响应辅助

从 `internal/handler/util.go` 中复制（非引用，因为 handler 包不应被插件依赖）：

```go
func WriteJSON(w http.ResponseWriter, status int, v any)
func WriteError(w http.ResponseWriter, status int, msg string)
```

所有插件改用这两个函数替换内联的 `http.Error` + 手动 `Set("Content-Type")` + `json.NewEncoder` 模式。

### 1.3 `validate.go` — ID 校验

```go
// ParseID 校验 res_id 是否为 32 字符，不合法返回错误
func ParseID(id string) error
```

替换所有插件中的 `if len(id) != 32 { ... }` 重复代码。

---

## 2. Store 层合并

### 2.1 `SubmissionStore` 接口变更

**删除** 3 个方法：
- `HasCorrectSubmissionInComp`
- `CreateSubmissionWithComp`
- `ListSubmissionsByComp`

**修改** 3 个现有方法，增加可选 `competitionID` 参数：

```go
type SubmissionStore interface {
    HasCorrectSubmission(ctx context.Context, userID, challengeID string, competitionID ...string) (bool, error)
    CreateSubmission(ctx context.Context, s *model.Submission) error
    ListSubmissions(ctx context.Context, params ListSubmissionsParams) ([]model.Submission, error)
    // ...
}
```

### 2.2 `ListSubmissionsParams` 结构体

```go
type ListSubmissionsParams struct {
    CompetitionID string // 可选，比赛范围过滤
    UserID        string // 可选，用户过滤
    ChallengeID   string // 可选，题目过滤
}
```

### 2.3 `mysql.go` 实现变更

**`HasCorrectSubmission`**：可选 `competitionID`，非空时追加 `AND competition_id=?` 条件。

**`CreateSubmission`**：统一为一个方法。`model.Submission.CompetitionID` 为指针类型（`*string`），nil 时不插入 competition_id 列，非 nil 时插入。用动态 SQL 拼接。

**`ListSubmissions`**：接收 `ListSubmissionsParams`，根据 `CompetitionID` 是否为空决定 WHERE 基础条件，其余动态拼接逻辑不变。

---

## 3. Handler 层变更

`internal/handler/submission.go`：

- `Submit` handler 调用 `store.HasCorrectSubmission(ctx, userID, challengeID)`（无 competitionID 参数）
- `SubmitInComp` handler 调用 `store.HasCorrectSubmission(ctx, userID, challengeID, compID)`（传 competitionID）
- `ListSubmissions` handler 调用 `store.ListSubmissions(ctx, ListSubmissionsParams{UserID: uid, ChallengeID: cid})`
- `ListSubmissionsByComp` handler 调用 `store.ListSubmissions(ctx, ListSubmissionsParams{CompetitionID: compID, UserID: uid, ChallengeID: cid})`

---

## 4. 插件层变更

### 4.1 leaderboard

- 用 `pluginutil.GetCompChallenges` 替换内联的 competition_challenges JOIN 查询，从中提取 ID 列表
- 用 `pluginutil.GetFirstCorrectSubmissions` 替换内联的正确提交查询
- 用 `pluginutil.GetUserScores` 替换内联的总分查询
- `topthree_records` 查询保留在插件内（私有表）
- 用 `pluginutil.WriteJSON`/`WriteError` 替换 `http.Error` + 手动 Content-Type

### 4.2 analytics

- **overview**: 用 `pluginutil.GetCompDistinctUsers`、`pluginutil.GetCompSubmitStats`、`pluginutil.GetCompChallengeCount` 替换内联查询
- **byCategory**: 用 `pluginutil.GetCompDistinctUsers`（查询一次，移到循环外）替换循环内的重复查询
- **challengeStats**: 用 `pluginutil.GetCompChallenges` 替换内联查询
- **userStats**: 此处是复杂聚合查询，不适合提取，保持内联
- 用 `pluginutil.WriteJSON`/`WriteError` 和 `pluginutil.ParseID` 替换重复代码

### 4.3 hints

- 用 `pluginutil.ParseID` 替换 4 处 `len(id) != 32` 检查
- 用 `pluginutil.WriteJSON`/`WriteError` 替换内联错误处理
- hints 表操作保留在插件内（私有表）

### 4.4 notification

- 用 `pluginutil.WriteJSON`/`WriteError` 替换内联错误处理
- notifications 表操作保留在插件内（私有表）

---

## 5. 不处理的项目

以下模式因为过于零散或改动收益不大，本次不处理：

- 空 slice nil-check 模式（6 处，分散且简单）
- Plugin struct 样板代码（4 处，属于各插件私有实现）
- Service 层 Submit/SubmitInComp 合并（用户明确要求不改动）
- analytics.userStats 和 analytics.challengeStats 中的复杂聚合查询（每次查询唯一，不适合提取）

---

## 6. 影响评估

| 改动 | 影响文件数 | 风险等级 |
|------|-----------|---------|
| 新增 pluginutil 包 | +3 新文件 | 低 |
| Store 接口变更 | 3 文件（store.go, mysql.go, service） | 中 |
| 插件改用 pluginutil | 4 文件 | 低 |
| Handler 层适配 | 1 文件 | 低 |

所有改动不改变外部 API（路由、请求/响应格式），回归测试可覆盖。
