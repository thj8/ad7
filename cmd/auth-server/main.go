// Package main 是认证服务的独立 HTTP 服务器入口。
// 提供用户注册/登录、JWT token 验证、队伍管理等功能。
// CTF 主服务通过 HTTP 调用 /api/v1/verify 来验证用户 token。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"ad7/internal/auth"
	"ad7/internal/config"
	"ad7/internal/logger"
	"ad7/internal/middleware"
	"ad7/internal/store"
)

func main() {
	cfgPath := flag.String("config", "cmd/auth-server/config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := logger.Init(cfg.Log); err != nil {
		log.Fatalf("init logger: %v", err)
	}

	st, err := store.New(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer st.Close()

	// auth 服务自己使用 JWT secret 做本地验证（用于保护自己的路由）
	authMW := middleware.NewAuth("http://localhost:"+fmt.Sprintf("%d", cfg.Server.Port), cfg.JWT.AdminRole)

	authStore := auth.NewAuthStore(st.DB())
	authSvc := auth.NewAuthService(authStore, cfg.JWT.Secret, cfg.JWT.AdminRole)
	teamSvc := auth.NewTeamService(authStore, authStore, authStore)
	authH := auth.NewAuthHandler(authSvc)
	teamH := auth.NewTeamHandler(teamSvc)
	verifyH := auth.NewVerifyHandler(authSvc)
	authDeps := auth.RouteDeps{
		Auth:       authMW,
		AuthH:      authH,
		TeamH:      teamH,
		AuthLimit:  cfg.RateLimit.Auth.Requests,
		AuthWindow: cfg.RateLimit.Auth.Window,
	}

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.MaxBodySize(1 << 20)) // 1MB body 限制

	// 公共路由（register, login, verify — 不需要 JWT）
	r.Route("/api/v1", func(r chi.Router) {
		auth.RegisterPublicRoutes(r, authDeps)
		r.Post("/verify", verifyH.Verify)

		// 需要认证的队伍路由
		r.Group(func(r chi.Router) {
			r.Use(authMW.Authenticate)
			auth.RegisterTeamRoutes(r, authDeps)
		})

		// 管理员队伍路由
		r.Route("/admin", func(r chi.Router) {
			r.Use(authMW.Authenticate)
			r.Use(authMW.RequireAdmin)
			auth.RegisterAdminTeamRoutes(r, authDeps)
		})
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	logger.Info("auth server starting", "port", cfg.Server.Port)
	log.Fatal(http.ListenAndServe(addr, r))
}
