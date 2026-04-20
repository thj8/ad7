package router

import (
	"github.com/go-chi/chi/v5"
)

// RegisterCompetitionRoutes 注册比赛公开路由。
// 公开路由：GET /competitions, GET /competitions/{id}, GET /competitions/{id}/challenges
func RegisterCompetitionRoutes(r chi.Router, deps RouteDeps) {
	r.Get("/competitions", deps.CompetitionH.List)
	r.Get("/competitions/{id}", deps.CompetitionH.Get)
	r.Get("/competitions/{id}/challenges", deps.CompetitionH.ListChallenges)
}

// registerAdminCompetitionRoutes 注册比赛 Admin 路由。
// Admin 路由：POST/PUT/DELETE /competitions/..., GET /competitions/{id}/submissions
func registerAdminCompetitionRoutes(r chi.Router, deps RouteDeps) {
	r.Get("/competitions/{id}/submissions", deps.SubmissionH.ListByComp)
	r.Post("/competitions", deps.CompetitionH.Create)
	r.Get("/competitions", deps.CompetitionH.ListAll)
	r.Put("/competitions/{id}", deps.CompetitionH.Update)
	r.Delete("/competitions/{id}", deps.CompetitionH.Delete)
	r.Post("/competitions/{id}/challenges", deps.CompetitionH.AddChallenge)
	r.Delete("/competitions/{id}/challenges/{challenge_id}", deps.CompetitionH.RemoveChallenge)
	r.Post("/competitions/{id}/start", deps.CompetitionH.Start)
	r.Post("/competitions/{id}/end", deps.CompetitionH.End)
}
