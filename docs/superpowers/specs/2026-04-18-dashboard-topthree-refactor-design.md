# Dashboard → TopThree 依赖重构设计

**日期**: 2026-04-18

## 背景

`plugins/dashboard/` 和 `plugins/topthree/` 存在一血追踪的职责重叠：

- **dashboard**: 在 `dashboard_first_blood` 表中独立追踪每个 challenge+competition 的首个正确提交
- **topthree**: 追踪前三个正确提交（ranking 1/2/3），其中 ranking=1 即为一血

两套独立的一血追踪机制导致数据可能不一致、维护成本高。

## 目标

1. Dashboard 不再自行管理一血数据，从 `topthree_records` 读取
2. 删除 `/firstblood` 端点
3. 排行榜增加详细的逐人逐题视图，标注是否解出及一二三血

## 变更清单

### 1. 删除 `/firstblood` 端点

**文件**: `plugins/dashboard/dashboard.go`
- 移除 `r.Get("/api/v1/dashboard/competitions/{id}/firstblood", p.getFirstBlood)`

**文件**: `plugins/dashboard/api.go`
- 删除 `getFirstBlood` handler 函数

**文件**: `plugins/dashboard/state.go`
- 删除 `getFirstBloodList` 函数

**文件**: `plugins/dashboard/model.go`
- 删除 `firstBlood` struct（仅被 `/firstblood` 端点使用）

### 2. 排行榜增加逐人逐题详情

**文件**: `plugins/dashboard/model.go`

新增 `challengeResult` 结构体，修改 `leaderboardEntry`：

```go
// challengeResult 表示用户在某道题目上的解题结果。
type challengeResult struct {
    ChallengeID string    `json:"challenge_id"`
    Solved      bool      `json:"solved"`
    BloodRank   int       `json:"blood_rank,omitempty"` // 1=一血, 2=二血, 3=三血, 未上榜不输出
    SolvedAt    time.Time `json:"solved_at,omitempty"`
}

// leaderboardEntry 表示排行榜中的一条记录。
type leaderboardEntry struct {
    Rank        int               `json:"rank"`
    UserID      string            `json:"user_id"`
    TotalScore  int               `json:"total_score"`
    LastSolveAt time.Time         `json:"last_solve_at"`
    Challenges  []challengeResult `json:"challenges"` // 新增：逐题解题详情
}
```

**文件**: `plugins/dashboard/state.go`

`getLeaderboard` 改为：
1. 查询该比赛所有题目列表（用于构造每个用户的 challengeResult）
2. 查询该比赛所有正确提交（user_id, challenge_id, created_at）
3. 查询 `topthree_records` 获取所有 ranking 1-3 的记录（user_id, challenge_id, ranking）
4. 组装每个用户的 `[]challengeResult`：每道题标记是否解出、解题时间、blood_rank

### 3. challengeState 的一血信息改为从 topthree_records 读取

**文件**: `plugins/dashboard/state.go`

`getChallengeStates` 第三步查询：
```sql
-- 旧
SELECT challenge_id, user_id, created_at
FROM dashboard_first_blood WHERE competition_id = ?

-- 新
SELECT challenge_id, user_id, created_at
FROM topthree_records
WHERE competition_id = ? AND ranking = 1 AND is_deleted = 0
```

`firstBloodInfo` struct 保留（用于 challengeState 中的一血显示）。

### 4. Dashboard 事件处理简化

**文件**: `plugins/dashboard/firstblood.go`

- 保留 `EventCorrectSubmission` 订阅（用于 recentEvents）
- 一血判断改为查询 `topthree_records` 中是否已有 ranking=1 记录
- 不再写入 `dashboard_first_blood` 表

### 5. 删除 dashboard_first_blood 表

**文件**: `plugins/dashboard/schema.sql`
- 删除整个 `CREATE TABLE` 语句

### 6. 集成测试更新

- 删除 `/firstblood` 端点相关测试
- 排行榜测试验证新增的 `challenges` 字段（解题状态、blood_rank）
- Dashboard 的测试数据需要 topthree 先写入排名数据

## 不变的部份

- Dashboard 的 `/state` 端点保持不变（响应结构有扩充）
- Dashboard 的 recentEvents 内存缓冲区保持不变
- Dashboard 的统计逻辑（stats）保持不变
- Topthree 插件完全不变

## /state 响应示例

```json
{
  "competition": { "id": "...", "title": "CTF 2026", ... },
  "challenges": [ ... ],
  "leaderboard": [
    {
      "rank": 1,
      "user_id": "alice",
      "total_score": 1500,
      "last_solve_at": "2026-04-18T10:30:00Z",
      "challenges": [
        { "challenge_id": "aaa", "solved": true,  "blood_rank": 1, "solved_at": "..." },
        { "challenge_id": "bbb", "solved": true,  "blood_rank": 2, "solved_at": "..." },
        { "challenge_id": "ccc", "solved": true,  "solved_at": "..." },
        { "challenge_id": "ddd", "solved": false }
      ]
    }
  ],
  "stats": { ... },
  "recent_events": [ ... ]
}
```

`blood_rank` 只在 1/2/3 时输出（omitempty），普通解题只有 `solved: true` + `solved_at`，未解出只有 `solved: false`。

## 风险

- **启动顺序**: topthree 必须在 dashboard 之前接收事件。需要在 `main.go` 中确保 topthree 先注册。
- **性能**: 排行榜查询从简单的聚合变为逐人逐题展开，数据量大时需关注。可用单次 JOIN 查询替代 N+1。
