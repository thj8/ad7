package router

import (
	"github.com/go-chi/chi/v5"

	"ad7/internal/ctxutil"
)

// RegisterCompetitionRoutes 注册比赛公开路由。
// 公开路由：GET /competitions, GET /competitions/{id}, GET /competitions/{id}/challenges
func RegisterCompetitionRoutes(r chi.Router, deps RouteDeps) {
	r.Get("/competitions", deps.CompetitionH.List)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Get("/competitions/{id}", deps.CompetitionH.Get)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Get("/competitions/{id}/challenges", deps.CompetitionH.ListChallenges)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Get("/competitions/{id}/teams", deps.CompetitionH.ListTeams)
}

// registerAdminCompetitionRoutes 注册比赛 Admin 路由。
// Admin 路由：POST/PUT/DELETE /competitions/..., GET /competitions/{id}/submissions
func registerAdminCompetitionRoutes(r chi.Router, deps RouteDeps) {
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Get("/competitions/{id}/submissions", deps.SubmissionH.ListByComp)
	r.Post("/competitions", deps.CompetitionH.Create)
	r.Get("/competitions", deps.CompetitionH.ListAll)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Put("/competitions/{id}", deps.CompetitionH.Update)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Delete("/competitions/{id}", deps.CompetitionH.Delete)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Post("/competitions/{id}/challenges", deps.CompetitionH.AddChallenge)
	r.With(
		ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID),
		ctxutil.ValidateURLParam("challenge_id", ctxutil.CtxKeyChalID),
	).Delete("/competitions/{id}/challenges/{challenge_id}", deps.CompetitionH.RemoveChallenge)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Post("/competitions/{id}/start", deps.CompetitionH.Start)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Post("/competitions/{id}/end", deps.CompetitionH.End)
	r.With(ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID)).
		Post("/competitions/{id}/teams", deps.CompetitionH.AddTeam)
	r.With(
		ctxutil.ValidateURLParam("id", ctxutil.CtxKeyID),
		ctxutil.ValidateURLParam("team_id", ctxutil.CtxKeyTeamID),
	).Delete("/competitions/{id}/teams/{team_id}", deps.CompetitionH.RemoveTeam)
}
