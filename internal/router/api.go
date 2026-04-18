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
