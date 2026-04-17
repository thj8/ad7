---
name: Dashboard Plugin Design
description: 大屏展示插件 - 一血检测 + 动态解题进度 API
type: spec
---

# Dashboard 插件设计文档

## 概述

CTF 平台大屏展示插件，提供一血检测和动态解题进度 API，供前端大屏展示使用。

## 需求

- 有人提交 flag 时判断是否是一血
- 动态解题进度通过 API 提供给大屏
- 以插件形式开发
- 仅一血需要持久化存储
- API 公开访问
- 实时性：轮询即可（10-30s）

## 架构

### 整体架构图

```
                    ┌─────────────────┐
                    │   Submission    │
                    │   Service       │
                    │  (Publish Event)│
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  Event Bus      │
                    │  (internal/)    │
                    └────────┬────────┘
                             │
                     ┌───────┴───────┐
                     ▼               ▼
            ┌──────────────┐  ┌──────────────┐
            │  Dashboard   │  │  (其他插件)  │
            │   Plugin     │  └──────────────┘
            └──────┬───────┘
                   │
                   ▼
            ┌──────────────┐
            │   HTTP API   │
            │  (公开访问)  │
            └──────────────┘
```

### 组件说明

| 组件 | 位置 | 职责 |
|------|------|------|
| Event Bus | `internal/event/` | 全局事件总线，支持订阅/发布 |
| Dashboard Plugin | `plugins/dashboard/` | 插件主入口，事件订阅，API 注册 |
| First Blood Detector | `plugins/dashboard/firstblood.go` | 一血检测与持久化 |
| State Aggregator | `plugins/dashboard/state.go` | 大屏状态聚合查询 |
| API Handlers | `plugins/dashboard/api.go` | HTTP API 处理器 |

## 数据设计

### 数据库表

新增表 `dashboard_first_blood`：

```sql
CREATE TABLE IF NOT EXISTS dashboard_first_blood (
    id INT AUTO_INCREMENT PRIMARY KEY,
    res_id BIGINT NOT NULL UNIQUE COMMENT '雪花ID',
    competition_id BIGINT NOT NULL COMMENT '比赛ID（0表示全局题）',
    challenge_id BIGINT NOT NULL COMMENT '题目ID',
    user_id VARCHAR(255) NOT NULL COMMENT '用户ID',
    created_at DATETIME NOT NULL COMMENT '一血时间',
    UNIQUE KEY idx_challenge_comp (challenge_id, competition_id),
    INDEX idx_competition (competition_id),
    INDEX idx_challenge (challenge_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='一血记录表';
```

### 内存状态

插件内部维护最近事件列表（仅内存，不持久化）：

```go
type recentEvent struct {
    Type          string    // "first_blood" or "solve"
    UserID        string
    ChallengeID   int64
    ChallengeTitle string
    Score         int       // 仅 solve 事件
    CreatedAt     time.Time
}

// 保留最近 100 条事件
```

## 事件机制

### Event Bus 设计

位置：`internal/event/event.go`

```go
package event

import (
    "context"
    "sync"
)

type EventType string

const (
    EventCorrectSubmission EventType = "correct_submission"
)

type Event struct {
    Type          EventType
    UserID        string
    ChallengeID   int64
    CompetitionID *int64  // 0 表示全局提交
    Ctx           context.Context
}

var (
    subscribers = make(map[EventType][]func(Event))
    mu          sync.RWMutex
)

// Subscribe 订阅事件
func Subscribe(t EventType, fn func(Event)) {
    mu.Lock()
    defer mu.Unlock()
    subscribers[t] = append(subscribers[t], fn)
}

// Publish 发布事件（异步通知订阅者）
func Publish(e Event) {
    mu.RLock()
    defer mu.RUnlock()
    for _, fn := range subscribers[e.Type] {
        go fn(e)
    }
}
```

### SubmissionService 集成

修改 `internal/service/submission.go`：

1. 在 `SubmissionService` 中引入 `event` 包
2. 在 `Submit()` 和 `SubmitInComp()` 中，确认提交正确后发布事件

```go
// 在 Submit() 中（全局题，competition_id = 0）
if isCorrect {
    var zeroID int64 = 0
    event.Publish(event.Event{
        Type:          event.EventCorrectSubmission,
        UserID:        userID,
        ChallengeID:   challengeID,
        CompetitionID: &zeroID,
        Ctx:           ctx,
    })
    return ResultCorrect, nil
}

// 在 SubmitInComp() 中（比赛内题目）
if isCorrect {
    event.Publish(event.Event{
        Type:          event.EventCorrectSubmission,
        UserID:        userID,
        ChallengeID:   challengeID,
        CompetitionID: &competitionID,
        Ctx:           ctx,
    })
    return ResultCorrect, nil
}
```

## 一血检测逻辑

### 检测流程

1. 收到 `EventCorrectSubmission` 事件
2. 查询该题目（在比赛中，如果有）是否已有一血记录
3. 若无，则原子性地插入一血记录
4. 同时添加到内存最近事件列表

### 原子性保证

使用 `INSERT ... WHERE NOT EXISTS` 保证并发安全：

```sql
INSERT INTO dashboard_first_blood (res_id, competition_id, challenge_id, user_id, created_at)
SELECT ?, ?, ?, ?, ?
WHERE NOT EXISTS (
    SELECT 1 FROM dashboard_first_blood
    WHERE challenge_id = ?
    AND (competition_id = ? OR (competition_id IS NULL AND ? IS NULL))
)
```

对于 MySQL，使用应用层判重 + 数据库唯一约束组合：
- 比赛内题目：`(challenge_id, competition_id)` 唯一索引
- 全局题目：约定 `competition_id = 0` 表示全局，建立 `(challenge_id, competition_id)` 唯一索引

## API 设计

### 公开 API（无需认证）

#### GET /api/v1/dashboard/competitions/{id}/state

获取比赛大屏完整状态。

**响应示例：**

```json
{
  "competition": {
    "id": 1234567890123456789,
    "title": "CTF 2026 春季赛",
    "is_active": true,
    "start_time": "2026-04-17T10:00:00Z",
    "end_time": "2026-04-17T22:00:00Z"
  },
  "challenges": [
    {
      "id": 1234567890123456789,
      "title": "Web Challenge 1",
      "category": "web",
      "score": 100,
      "solve_count": 15,
      "first_blood": {
        "user_id": "user1",
        "created_at": "2026-04-17T10:05:00Z"
      }
    }
  ],
  "leaderboard": [
    {
      "rank": 1,
      "user_id": "user1",
      "total_score": 500,
      "last_solve_at": "2026-04-17T12:30:00Z"
    },
    {
      "rank": 2,
      "user_id": "user2",
      "total_score": 400,
      "last_solve_at": "2026-04-17T12:15:00Z"
    }
  ],
  "stats": {
    "total_users": 100,
    "total_solves": 250,
    "solves_by_category": {
      "web": 80,
      "pwn": 60,
      "crypto": 50,
      "rev": 40,
      "misc": 20
    }
  },
  "recent_events": [
    {
      "type": "first_blood",
      "user_id": "user1",
      "challenge_id": 1234567890123456789,
      "challenge_title": "Web Challenge 1",
      "created_at": "2026-04-17T10:05:00Z"
    },
    {
      "type": "solve",
      "user_id": "user2",
      "challenge_id": 1234567890123456790,
      "challenge_title": "Pwn Challenge 1",
      "score": 200,
      "created_at": "2026-04-17T10:10:00Z"
    }
  ]
}
```

#### GET /api/v1/dashboard/competitions/{id}/firstblood

获取比赛一血列表。

**响应示例：**

```json
[
  {
    "challenge_id": 1234567890123456789,
    "challenge_title": "Web Challenge 1",
    "category": "web",
    "score": 100,
    "user_id": "user1",
    "created_at": "2026-04-17T10:05:00Z"
  }
]
```

## 插件结构

```
plugins/dashboard/
├── dashboard.go    # 插件入口，Register 方法，事件订阅
├── api.go          # HTTP handlers
├── state.go        # 状态聚合逻辑
├── firstblood.go   # 一血检测与存储
├── model.go        # 内部数据模型
└── schema.sql      # 数据库表定义
```

### dashboard.go

实现 `plugin.Plugin` 接口：

```go
type Plugin struct {
    db           *sql.DB
    recentEvents []recentEvent
    mu           sync.RWMutex
}

func New() *Plugin {
    return &Plugin{
        recentEvents: make([]recentEvent, 0, 100),
    }
}

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
    p.db = db

    // 订阅正确提交事件
    event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

    // 注册 API 路由
    r.Get("/api/v1/dashboard/competitions/{id}/state", p.getState)
    r.Get("/api/v1/dashboard/competitions/{id}/firstblood", p.getFirstBlood)
}
```

### firstblood.go

处理一血检测：

```go
func (p *Plugin) handleCorrectSubmission(e event.Event) {
    // 1. 查询是否已有一血
    // 2. 若无，原子性插入
    // 3. 添加到 recentEvents
}
```

### state.go

聚合大屏状态：

```go
func (p *Plugin) getCompetitionState(ctx context.Context, compID int64) (*State, error) {
    // 查询比赛信息
    // 查询题目列表及解题数
    // 查询排行榜（复用 leaderboard 插件逻辑）
    // 统计各分类解题数
    // 返回聚合状态
}
```

## 集成到 main.go

在 `cmd/server/main.go` 中：

```go
// 导入插件
import (
    // ...
    "ad7/plugins/dashboard"
)

func main() {
    // ... 现有初始化代码 ...

    // 注册插件
    plugins := []plugin.Plugin{
        leaderboard.New(),
        notification.New(),
        dashboard.New(),  // 新增
    }
    for _, p := range plugins {
        p.Register(r, st.DB(), auth)
    }

    // ... 启动服务器 ...
}
```

## 测试计划

### 单元测试

- Event Bus 订阅/发布测试
- 一血检测并发安全测试
- API handler 测试

### 集成测试

- 完整提交流程 → 一血记录测试
- API 响应格式测试
- 比赛内 vs 全局题一血测试

## 约束与注意事项

1. **不使用外键**：遵循项目约束
2. **Snowflake ID**：所有公开 ID 使用 snowflake
3. **并发安全**：一血检测需要处理并发提交场景
4. **性能**：`/state` 接口尽量使用单次查询或少量查询，避免 N+1
5. **无前端**：本插件仅提供 API，不包含前端实现
