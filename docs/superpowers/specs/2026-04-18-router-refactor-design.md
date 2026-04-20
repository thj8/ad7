---
name: Router Refactor Design
description: 重构 main.go 路由注册，将路由拆分到 internal/router 包按模块组织
type: design
---

# Router 重构设计文档

## 概述

将 `cmd/server/main.go` 第 74-116 行的路由注册逻辑拆分到独立的 `internal/router` 包中，按功能模块组织路由注册，避免 main.go 随着功能增加变得臃肿。

## 目录结构

```
internal/router/
├── api.go           # 统一入口 RegisterAPIV1 + RouteDeps 结构体
├── challenges.go    # RegisterChallengeRoutes
├── competitions.go  # RegisterCompetitionRoutes
└── submissions.go   # RegisterSubmissionRoutes
```

## 设计详情

### 1. RouteDeps 结构体

封装路由注册所需的所有依赖，符合项目"函数参数不超过4个"的约束。

```go
type RouteDeps struct {
	Auth         *middleware.Auth
	Config       *config.Config
	ChallengeH   *handler.ChallengeHandler
	CompetitionH *handler.CompetitionHandler
	SubmissionH  *handler.SubmissionHandler
}
```

### 2. 统一入口 RegisterAPIV1

创建 `/api/v1` 子路由组，统一加 `auth.Authenticate` 中间件，然后调用各模块注册函数。

```go
func RegisterAPIV1(r chi.Router, deps RouteDeps)
```

### 3. 模块路由注册函数

每个功能模块有自己的注册函数：

- `RegisterChallengeRoutes(r chi.Router, deps RouteDeps)`
- `RegisterCompetitionRoutes(r chi.Router, deps RouteDeps)`
- `RegisterSubmissionRoutes(r chi.Router, deps RouteDeps)`

每个函数负责：
- 注册自己模块的公开路由
- 注册自己模块的 admin 路由（带 `auth.RequireAdmin` 中间件）

### 4. main.go 的变化

删除原第 74-116 行的直接路由注册，替换为：

```go
router.RegisterAPIV1(r, router.RouteDeps{
    Auth:         auth,
    Config:       cfg,
    ChallengeH:   challengeH,
    CompetitionH: compH,
    SubmissionH:  submissionH,
})
```

插件注册部分（第 118-128 行）保持不变。

## 路由映射

| 路径 | Handler | 中间件 | 所在模块 |
|------|---------|--------|----------|
| GET /api/v1/challenges | challengeH.List | auth.Authenticate | challenges |
| GET /api/v1/challenges/{id} | challengeH.Get | auth.Authenticate | challenges |
| POST /api/v1/admin/challenges | challengeH.Create | auth.Authenticate + auth.RequireAdmin | challenges |
| PUT /api/v1/admin/challenges/{id} | challengeH.Update | auth.Authenticate + auth.RequireAdmin | challenges |
| DELETE /api/v1/admin/challenges/{id} | challengeH.Delete | auth.Authenticate + auth.RequireAdmin | challenges |
| GET /api/v1/competitions | compH.List | auth.Authenticate | competitions |
| GET /api/v1/competitions/{id} | compH.Get | auth.Authenticate | competitions |
| GET /api/v1/competitions/{id}/challenges | compH.ListChallenges | auth.Authenticate | competitions |
| POST /api/v1/competitions/{comp_id}/challenges/{id}/submit | submissionH.SubmitInComp | auth.Authenticate + LimitByUserID | submissions |
| GET /api/v1/admin/competitions/{id}/submissions | submissionH.ListByComp | auth.Authenticate + auth.RequireAdmin | competitions |
| POST /api/v1/admin/competitions | compH.Create | auth.Authenticate + auth.RequireAdmin | competitions |
| GET /api/v1/admin/competitions | compH.ListAll | auth.Authenticate + auth.RequireAdmin | competitions |
| PUT /api/v1/admin/competitions/{id} | compH.Update | auth.Authenticate + auth.RequireAdmin | competitions |
| DELETE /api/v1/admin/competitions/{id} | compH.Delete | auth.Authenticate + auth.RequireAdmin | competitions |
| POST /api/v1/admin/competitions/{id}/challenges | compH.AddChallenge | auth.Authenticate + auth.RequireAdmin | competitions |
| DELETE /api/v1/admin/competitions/{id}/challenges/{challenge_id} | compH.RemoveChallenge | auth.Authenticate + auth.RequireAdmin | competitions |

## 测试策略

- 运行现有集成测试确保路由正常工作
- 不改变任何 handler 逻辑，只移动路由注册代码
