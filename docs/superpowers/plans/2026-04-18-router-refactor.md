# Router Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 main.go 中的路由注册逻辑拆分到独立的 internal/router 包中，按功能模块组织路由注册

**Architecture:** 创建 internal/router 包，包含 RouteDeps 结构体和统一入口 RegisterAPIV1，按模块拆分路由注册函数

**Tech Stack:** Go, chi router

---

## File Structure

**New files:**
- `internal/router/api.go` - RouteDeps 结构体 + RegisterAPIV1 统一入口
- `internal/router/challenges.go` - 题目路由注册
- `internal/router/competitions.go` - 比赛路由注册
- `internal/router/submissions.go` - 提交路由注册

**Modified files:**
- `cmd/server/main.go` - 删除路由注册代码，替换为 router.RegisterAPIV1 调用

---

### Task 1: Create router package and api.go

**Files:**
- Create: `internal/router/api.go`

- [ ] **Step 1: Create the router package directory**

```bash
mkdir -p /Users/sugar/src/project/ad7/internal/router
```

- [ ] **Step 2: Write api.go with RouteDeps and RegisterAPIV1**

```go
// Package router 负责 API 路由的集中注册。
// 将 main.go 中的路由注册逻辑拆分到这里，按模块组织。
package router

import (
	"github.com/go-chi/chi/v5"

	"ad7/internal/config"
	"ad7/internal/handler"
	"ad7/internal/middleware"
)

// RouteDeps 封装路由注册所需的所有依赖。
// 用结构体传参符合项目"函数参数不超过4个"的约束。
type RouteDeps struct {
	Auth         *middleware.Auth
	Config       *config.Config
	ChallengeH   *handler.ChallengeHandler
	CompetitionH *handler.CompetitionHandler
	SubmissionH  *handler.SubmissionHandler
}

// RegisterAPIV1 注册所有 /api/v1 路由。
// 创建 /api/v1 子路由组，统一加 auth.Authenticate 中间件，
// 然后调用各模块的注册函数。
func RegisterAPIV1(r chi.Router, deps RouteDeps) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(deps.Auth.Authenticate)

		RegisterChallengeRoutes(r, deps)
		RegisterCompetitionRoutes(r, deps)
		RegisterSubmissionRoutes(r, deps)
	})
}
```

- [ ] **Step 3: Verify file compiles**

```bash
go build ./internal/router
```
Expected: no errors (will have "undefined" warnings for the register functions, that's expected)

- [ ] **Step 4: Commit**

```bash
git add internal/router/api.go
git commit -m "feat(router): add router package with api.go"
```

---

### Task 2: Create challenges.go

**Files:**
- Create: `internal/router/challenges.go`

- [ ] **Step 1: Write challenges.go with RegisterChallengeRoutes**

```go
package router

import (
	"github.com/go-chi/chi/v5"
)

// RegisterChallengeRoutes 注册题目相关路由。
// 公开路由：GET /challenges, GET /challenges/{id}
// Admin 路由：POST/PUT/DELETE /admin/challenges/...
func RegisterChallengeRoutes(r chi.Router, deps RouteDeps) {
	// 公开路由
	r.Get("/challenges", deps.ChallengeH.List)
	r.Get("/challenges/{id}", deps.ChallengeH.Get)

	// Admin 路由
	r.Route("/admin", func(r chi.Router) {
		r.Use(deps.Auth.RequireAdmin)
		r.Post("/challenges", deps.ChallengeH.Create)
		r.Put("/challenges/{id}", deps.ChallengeH.Update)
		r.Delete("/challenges/{id}", deps.ChallengeH.Delete)
	})
}
```

- [ ] **Step 2: Verify file compiles**

```bash
go build ./internal/router
```
Expected: no errors (will have warnings for competitions/submissions, that's expected)

- [ ] **Step 3: Commit**

```bash
git add internal/router/challenges.go
git commit -m "feat(router): add challenge routes"
```

---

### Task 3: Create competitions.go

**Files:**
- Create: `internal/router/competitions.go`

- [ ] **Step 1: Write competitions.go with RegisterCompetitionRoutes**

```go
package router

import (
	"github.com/go-chi/chi/v5"
)

// RegisterCompetitionRoutes 注册比赛相关路由。
// 公开路由：GET /competitions, GET /competitions/{id}, GET /competitions/{id}/challenges
// Admin 路由：POST/PUT/DELETE /admin/competitions/...
func RegisterCompetitionRoutes(r chi.Router, deps RouteDeps) {
	// 公开路由
	r.Get("/competitions", deps.CompetitionH.List)
	r.Get("/competitions/{id}", deps.CompetitionH.Get)
	r.Get("/competitions/{id}/challenges", deps.CompetitionH.ListChallenges)

	// Admin 路由
	r.Route("/admin", func(r chi.Router) {
		r.Use(deps.Auth.RequireAdmin)
		r.Get("/competitions/{id}/submissions", deps.SubmissionH.ListByComp)
		r.Post("/competitions", deps.CompetitionH.Create)
		r.Get("/competitions", deps.CompetitionH.ListAll)
		r.Put("/competitions/{id}", deps.CompetitionH.Update)
		r.Delete("/competitions/{id}", deps.CompetitionH.Delete)
		r.Post("/competitions/{id}/challenges", deps.CompetitionH.AddChallenge)
		r.Delete("/competitions/{id}/challenges/{challenge_id}", deps.CompetitionH.RemoveChallenge)
	})
}
```

- [ ] **Step 2: Verify file compiles**

```bash
go build ./internal/router
```
Expected: no errors (will have warning for submissions, that's expected)

- [ ] **Step 3: Commit**

```bash
git add internal/router/competitions.go
git commit -m "feat(router): add competition routes"
```

---

### Task 4: Create submissions.go

**Files:**
- Create: `internal/router/submissions.go`

- [ ] **Step 1: Write submissions.go with RegisterSubmissionRoutes**

```go
package router

import (
	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

// RegisterSubmissionRoutes 注册提交相关路由。
// 路由：POST /competitions/{comp_id}/challenges/{id}/submit
func RegisterSubmissionRoutes(r chi.Router, deps RouteDeps) {
	r.With(
		middleware.LimitByUserID(
			deps.Config.RateLimit.Submission.Requests,
			deps.Config.RateLimit.Submission.Window,
		),
	).Post("/competitions/{comp_id}/challenges/{id}/submit", deps.SubmissionH.SubmitInComp)
}
```

- [ ] **Step 2: Verify file compiles**

```bash
go build ./internal/router
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/router/submissions.go
git commit -m "feat(router): add submission routes"
```

---

### Task 5: Update main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add router import**

Add to imports:
```go
	"ad7/internal/router"
```

- [ ] **Step 2: Replace routing section (lines 74-116) with router.RegisterAPIV1 call**

Delete lines 74-116 and replace with:
```go
	// 注册 API v1 路由组（通过 router 包统一注册）
	router.RegisterAPIV1(r, router.RouteDeps{
		Auth:         auth,
		Config:       cfg,
		ChallengeH:   challengeH,
		CompetitionH: compH,
		SubmissionH:  submissionH,
	})
```

- [ ] **Step 3: Verify the full main.go compiles**

```bash
go build ./cmd/server
```
Expected: no errors

- [ ] **Step 4: Run tests to verify everything works**

```bash
go test ./...
```
Expected: all tests pass

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "refactor: move routing to internal/router package"
```

---

### Task 6: Verify integration tests pass

**Files:**
- Test: `internal/integration/integration_test.go`

- [ ] **Step 1: Run integration tests (requires MySQL)**

```bash
go test ./internal/integration/... -v -count=1
```
Expected: all 16 integration tests pass

- [ ] **Step 2: Verify no changes needed**

(No code changes needed - this is just verification)

---

## Self-Review Checklist

- [x] Spec coverage: All requirements from spec covered
- [x] Placeholder scan: No TBD/TODO/placeholders
- [x] Type consistency: All type names, function signatures match
