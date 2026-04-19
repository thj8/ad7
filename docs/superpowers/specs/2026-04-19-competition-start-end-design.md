# 比赛开始/结束操作设计

## 概述

为比赛添加自动和手动的状态切换能力。基于已有的 `start_time`/`end_time` 字段，通过惰性检查实现自动开始/结束，同时提供管理员手动控制 API。

## 自动状态检查（惰性）

在 service 层的查询方法中，查询到比赛数据后检查时间并自动更新状态：

**激活条件：** `start_time <= now && end_time > now && is_active == false` → 设置 `is_active = true`

**结束条件：** `end_time <= now && is_active == true` → 设置 `is_active = false`

**触发位置：** `service/competition.go` 中所有读取比赛的方法调用前/后。

**实现方式：** 新增 `syncStatus(ctx, *model.Competition)` 私有方法，在 `Get`、`ListActive` 返回结果时调用。`List`（管理员列表）和 `ListActive` 对批量结果逐条检查。

为避免频繁写库，仅在状态实际变更时才执行 UPDATE。

## 手动控制 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/admin/competitions/{id}/start` | 强制开始比赛 |
| POST | `/api/v1/admin/competitions/{id}/end` | 强制结束比赛 |

需要 JWT 认证 + 管理员权限。返回更新后的比赛信息。

请求/响应：
- 请求：无 body
- 响应：`200 OK` + 比赛 JSON（同 Get 接口格式）
- 错误：`404` 比赛不存在，`409` 比赛已处于目标状态

## Service 层新增

```go
// StartCompetition 手动开始比赛，设置 is_active = true
func (s *CompetitionService) StartCompetition(ctx context.Context, resID string) (*model.Competition, error)

// EndCompetition 手动结束比赛，设置 is_active = false
func (s *CompetitionService) EndCompetition(ctx context.Context, resID string) (*model.Competition, error)
```

**冲突处理：**
- `StartCompetition` 如果比赛已是 `is_active=true`，返回 `409 Conflict`
- `EndCompetition` 如果比赛已是 `is_active=false`，返回 `409 Conflict`

## Store 层新增

```go
// SetActive 设置比赛的 is_active 状态
func (s *Store) SetActive(ctx context.Context, resID string, active bool) error
```

单条 UPDATE 语句，WHERE 条件包含 `is_deleted = 0`。

## Handler 层新增

在 `internal/handler/competition.go` 中新增两个方法：
- `Start(w, r)` — 调用 `svc.StartCompetition`，返回比赛信息
- `End(w, r)` — 调用 `svc.EndCompetition`，返回比赛信息

在 `internal/router/competitions.go` 中注册路由，使用 `auth.RequireAdmin` 保护。

## 日志

使用已有的 `logger` 包记录操作：
- 手动开始：`logger.Info("competition started", "user", ..., "competition_id", ...)`
- 手动结束：`logger.Info("competition ended", "user", ..., "competition_id", ...)`
- 自动状态变更：`logger.Info("competition auto-activated", "competition_id", ...)` / `logger.Info("competition auto-ended", ...)`

## 测试

- 单元测试：`SetActive` store 方法
- 集成测试：
  - 测试手动开始/结束 API（POST /admin/competitions/{id}/start, /end）
  - 测试 409 重复操作冲突
  - 测试惰性检查：创建一个 start_time 在过去、is_active=false 的比赛，调用 ListActive 后验证其自动激活
  - 测试惰性检查：创建一个 end_time 在过去、is_active=true 的比赛，调用 Get 后验证其自动结束
