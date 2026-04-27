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
		middleware.ValidateURLParam("comp_id", middleware.CtxKeyCompID),
		middleware.ValidateURLParam("id", middleware.CtxKeyChalID),
	).Post("/competitions/{comp_id}/challenges/{id}/submit", deps.SubmissionH.SubmitInComp)
}
