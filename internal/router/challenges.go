package router

import (
	"github.com/go-chi/chi/v5"
)

// RegisterChallengeRoutes 注册题目公开路由。
// 公开路由：GET /challenges, GET /challenges/{id}
func RegisterChallengeRoutes(r chi.Router, deps RouteDeps) {
	r.Get("/challenges", deps.ChallengeH.List)
	r.Get("/challenges/{id}", deps.ChallengeH.Get)
}

// registerAdminChallengeRoutes 注册题目 Admin 路由。
// Admin 路由：POST/PUT/DELETE /challenges/...
func registerAdminChallengeRoutes(r chi.Router, deps RouteDeps) {
	r.Post("/challenges", deps.ChallengeH.Create)
	r.Put("/challenges/{id}", deps.ChallengeH.Update)
	r.Delete("/challenges/{id}", deps.ChallengeH.Delete)
}
