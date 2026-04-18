---
name: Top Three Plugin Design
description: 每道题前三名答题信息插件 - 预计算存储 + API 提供
type: spec
---

# Top Three 插件设计文档

## 概述

CTF 平台每道题前三名答题信息插件，通过事件监听预计算并存储每道题的前三名，提供 API 供前端展示。

## 需求

- 返回一场比赛中每一个题目的前三名答题信息
- 以插件形式开发，不修改主流程
- 预计算存储（新建表），非实时查询
- API 需要登录用户访问
- 一道题目一个用户只能有一次正确提交
- 按解题时间最早排序前三名

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
            │  TopThree    │  │  (其他插件)  │
            │   Plugin     │  └──────────────┘
            └──────┬───────┘
                   │
                   ▼
            ┌──────────────┐
            │   HTTP API   │
            │(需登录访问)  │
            └──────────────┘
```

### 组件说明

| 组件 | 位置 | 职责 |
|------|------|------|
| Event Bus | `internal/event/` | 全局事件总线，支持订阅/发布 |
| TopThree Plugin | `plugins/topthree/` | 插件主入口，事件订阅，API 注册 |
| Top Three Manager | `plugins/topthree/topthree.go` | 前三名检测与持久化 |
| API Handlers | `plugins/topthree/api.go` | HTTP API 处理器 |

## 数据设计

### 数据库表

新增表 `topthree_records`：

```sql
CREATE TABLE IF NOT EXISTS topthree_records (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32)  NOT NULL UNIQUE COMMENT 'UUID',
    competition_id VARCHAR(32)  NOT NULL COMMENT '比赛ID',
    challenge_id   VARCHAR(32)  NOT NULL COMMENT '题目ID',
    user_id        VARCHAR(128) NOT NULL COMMENT '用户ID',
    rank           TINYINT      NOT NULL COMMENT '排名 1-3',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '解题时间',
    UNIQUE INDEX idx_comp_chal_rank (competition_id, challenge_id, rank),
    INDEX idx_comp_chal (competition_id, challenge_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每道题前三名记录表';
```

## 事件机制

复用现有 `EventCorrectSubmission` 事件，详见 dashboard 插件设计文档。

## 前三名更新逻辑

### 更新流程

1. 收到 `EventCorrectSubmission` 事件
2. 若比赛ID为空，忽略（仅处理比赛内提交）
3. 查询该题在该比赛中的现有前三名
4. 检查当前用户是否已在前三名中，若是则不处理
5. 检查当前提交时间是否能进入前三名：
   - 若前三名未满（<3条），直接插入
   - 若前三名已满，比较时间，比第3名快则替换
6. 调整后续排名（如需要）

### 并发安全

使用数据库唯一约束 `idx_comp_chal_rank` 保证同一比赛同一题目同一排名只有一条记录。

## API 设计

### 需要认证的 API

#### GET /api/v1/topthree/competitions/{id}

获取比赛每道题前三名信息。

**响应示例：**

```json
{
  "competition_id": "abc123def456",
  "challenges": [
    {
      "challenge_id": "xyz789uvw012",
      "title": "Web Challenge 1",
      "category": "web",
      "score": 100,
      "top_three": [
        {
          "rank": 1,
          "user_id": "user1",
          "created_at": "2026-04-18T10:00:00Z"
        },
        {
          "rank": 2,
          "user_id": "user2",
          "created_at": "2026-04-18T10:01:00Z"
        },
        {
          "rank": 3,
          "user_id": "user3",
          "created_at": "2026-04-18T10:02:00Z"
        }
      ]
    }
  ]
}
```

## 插件结构

```
plugins/topthree/
├── topthree.go   # 插件入口，Register 方法，事件订阅，前三名逻辑
├── api.go        # HTTP handlers
├── model.go      # 内部数据模型
└── schema.sql    # 数据库表定义
```

### topthree.go

实现 `plugin.Plugin` 接口：

```go
type Plugin struct {
    db *sql.DB
}

func New() *Plugin {
    return &Plugin{}
}

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
    p.db = db

    // 订阅正确提交事件
    event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

    // 注册 API 路由（需要认证）
    r.Group(func(r chi.Router) {
        r.Use(auth.Authenticate)
        r.Get("/api/v1/topthree/competitions/{id}", p.getTopThree)
    })
}
```

### 前三名更新逻辑

```go
func (p *Plugin) handleCorrectSubmission(e event.Event) {
    // 1. 检查是否有比赛ID，无则忽略
    // 2. 查询该题当前前三名
    // 3. 检查用户是否已在前三名中
    // 4. 判断是否能进入前三名
    // 5. 更新数据库（调整排名、插入新记录、删除被挤出的记录）
}
```

## 集成到 main.go

在 `cmd/server/main.go` 中：

```go
// 导入插件
import (
    // ...
    "ad7/plugins/topthree"
)

func main() {
    // ... 现有初始化代码 ...

    // 注册插件
    plugins := []plugin.Plugin{
        leaderboard.New(),
        notification.New(),
        analytics.New(),
        dashboard.New(),
        hints.New(),
        topthree.New(),  // 新增
    }
    for _, p := range plugins {
        p.Register(r, st.DB(), auth)
    }

    // ... 启动服务器 ...
}
```

## 测试计划

### 集成测试

- 单个题目前三名顺序提交测试
- 并发提交测试（验证排名正确）
- 后来提交但时间更快的替换测试
- API 响应格式测试
- 非比赛内提交忽略测试

## 约束与注意事项

1. **不使用外键**：遵循项目约束
2. **UUID res_id**：所有公开 ID 使用 UUID v4（32字符无连字符）
3. **并发安全**：使用数据库唯一约束保证排名不重复
4. **软删除**：遵循项目模式，使用 `is_deleted` 字段（本表暂不需要）
5. **仅比赛内**：仅处理有 `competition_id` 的提交
