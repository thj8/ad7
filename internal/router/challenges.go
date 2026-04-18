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
