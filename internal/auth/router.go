package auth

import (
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

// RouteDeps 封装认证和队伍路由注册所需的依赖。
type RouteDeps struct {
	Auth       *middleware.Auth
	AuthH      *AuthHandler
	TeamH      *TeamHandler
	AuthLimit  int           // 认证端点限流请求数
	AuthWindow time.Duration // 认证端点限流时间窗口
}

// RegisterPublicRoutes 注册不需要认证的公共路由（带限流）。
func RegisterPublicRoutes(r chi.Router, deps RouteDeps) {
	r.Group(func(r chi.Router) {
		if deps.AuthLimit > 0 && deps.AuthWindow > 0 {
			r.Use(middleware.LimitByIP(deps.AuthLimit, deps.AuthWindow))
		}
		r.Post("/register", deps.AuthH.Register)
		r.Post("/login", deps.AuthH.Login)
	})
}

// RegisterTeamRoutes 注册需要认证的队伍路由。
// 路由：GET /teams, GET /teams/{id}, GET /teams/{id}/members
func RegisterTeamRoutes(r chi.Router, deps RouteDeps) {
	r.Get("/teams", deps.TeamH.List)
	r.Get("/teams/{id}", deps.TeamH.Get)
	r.Get("/teams/{id}/members", deps.TeamH.ListMembers)
}

// RegisterAdminTeamRoutes 注册管理员队伍路由。
// 路由：POST/PUT/DELETE /teams, POST/DELETE /teams/{id}/members, PUT /teams/{id}/captain, POST /teams/{id}/transfer-captain
func RegisterAdminTeamRoutes(r chi.Router, deps RouteDeps) {
	r.Post("/teams", deps.TeamH.Create)
	r.Put("/teams/{id}", deps.TeamH.Update)
	r.Delete("/teams/{id}", deps.TeamH.Delete)
	r.Post("/teams/{id}/members", deps.TeamH.AddMember)
	r.Delete("/teams/{id}/members/{user_id}", deps.TeamH.RemoveMember)
	r.Put("/teams/{id}/captain", deps.TeamH.SetCaptain)
	r.Post("/teams/{id}/transfer-captain", deps.TeamH.TransferCaptain)
}
