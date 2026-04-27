package router

import (
	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

// RegisterCompetitionRoutes 注册比赛公开路由。
// 公开路由：GET /competitions, GET /competitions/{id}, GET /competitions/{id}/challenges
func RegisterCompetitionRoutes(r chi.Router, deps RouteDeps) {
	r.Get("/competitions", deps.CompetitionH.List)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Get("/competitions/{id}", deps.CompetitionH.Get)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Get("/competitions/{id}/challenges", deps.CompetitionH.ListChallenges)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Get("/competitions/{id}/teams", deps.CompetitionH.ListTeams)
}

// registerAdminCompetitionRoutes 注册比赛 Admin 路由。
// Admin 路由：POST/PUT/DELETE /competitions/..., GET /competitions/{id}/submissions
func registerAdminCompetitionRoutes(r chi.Router, deps RouteDeps) {
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Get("/competitions/{id}/submissions", deps.SubmissionH.ListByComp)
	r.Post("/competitions", deps.CompetitionH.Create)
	r.Get("/competitions", deps.CompetitionH.ListAll)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Put("/competitions/{id}", deps.CompetitionH.Update)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Delete("/competitions/{id}", deps.CompetitionH.Delete)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Post("/competitions/{id}/challenges", deps.CompetitionH.AddChallenge)
	r.With(
		middleware.ValidateURLParam("id", middleware.CtxKeyID),
		middleware.ValidateURLParam("challenge_id", middleware.CtxKeyChalID),
	).Delete("/competitions/{id}/challenges/{challenge_id}", deps.CompetitionH.RemoveChallenge)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Post("/competitions/{id}/start", deps.CompetitionH.Start)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Post("/competitions/{id}/end", deps.CompetitionH.End)
	r.With(middleware.ValidateURLParam("id", middleware.CtxKeyID)).
		Post("/competitions/{id}/teams", deps.CompetitionH.AddTeam)
	r.With(
		middleware.ValidateURLParam("id", middleware.CtxKeyID),
		middleware.ValidateURLParam("team_id", middleware.CtxKeyTeamID),
	).Delete("/competitions/{id}/teams/{team_id}", deps.CompetitionH.RemoveTeam)
}
