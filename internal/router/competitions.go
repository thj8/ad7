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
