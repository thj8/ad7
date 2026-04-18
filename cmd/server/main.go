// Package main 是 CTF 比赛平台的 HTTP 服务器入口。
// 负责加载配置、初始化数据库连接、组装各层组件、注册路由和插件，最后启动 HTTP 服务。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"ad7/internal/config"
	"ad7/internal/handler"
	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/service"
	"ad7/internal/store"
	"ad7/plugins/analytics"

	"ad7/plugins/hints"
	"ad7/plugins/leaderboard"
	"ad7/plugins/notification"
	"ad7/plugins/topthree"
)

// main 是程序入口。按以下顺序初始化并启动服务：
// 1. 解析命令行参数（-config 指定配置文件路径，默认 config.yaml）
// 2. 加载配置文件
// 3. 连接数据库
// 4. 创建认证中间件
// 5. 初始化 Service 层（题目、提交、比赛）
// 6. 初始化 Handler 层
// 7. 注册 chi 路由（含 JWT 认证和管理员权限校验）
// 8. 加载并注册所有插件
// 9. 启动 HTTP 服务器
func main() {
	// 解析配置文件路径参数
	cfgPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	// 加载 YAML 配置文件，包含服务器端口、数据库连接信息、JWT 密钥等
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 通过 DSN 连接 MySQL 数据库
	st, err := store.New(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer st.Close()

	// 创建 JWT 认证中间件，传入密钥和管理员角色名
	auth := middleware.NewAuth(cfg.JWT.Secret, cfg.JWT.AdminRole)

	// 初始化 Service 层，注入对应的 Store 接口
	challengeSvc := service.NewChallengeService(st)
	submissionSvc := service.NewSubmissionService(st, st)
	compSvc := service.NewCompetitionService(st)

	// 初始化 Handler 层，注入对应的 Service
	challengeH := handler.NewChallengeHandler(challengeSvc)
	submissionH := handler.NewSubmissionHandler(submissionSvc)
	compH := handler.NewCompetitionHandler(compSvc)

	// 创建 chi 路由器并挂载全局中间件
	r := chi.NewRouter()
	r.Use(chimw.Logger)    // 请求日志记录
	r.Use(chimw.Recoverer) // panic 恢复，防止服务崩溃

	// 注册 API v1 路由组
	r.Route("/api/v1", func(r chi.Router) {
		// 所有 /api/v1 路由都需要 JWT 认证
		r.Use(auth.Authenticate)

		// 公开路由：题目列表、题目详情
		r.Get("/challenges", challengeH.List)
		r.Get("/challenges/{id}", challengeH.Get)

		// 公开路由：比赛列表、比赛详情、比赛下的题目列表、比赛内提交 Flag
		r.Get("/competitions", compH.List)
		r.Get("/competitions/{id}", compH.Get)
		r.Get("/competitions/{id}/challenges", compH.ListChallenges)
		r.Post("/competitions/{comp_id}/challenges/{id}/submit", submissionH.SubmitInComp)

		// 管理员路由：需要 admin 角色
		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireAdmin)

			// 题目管理：创建、更新、删除
			r.Post("/challenges", challengeH.Create)
			r.Put("/challenges/{id}", challengeH.Update)
			r.Delete("/challenges/{id}", challengeH.Delete)

			// 提交记录查询
			r.Get("/competitions/{id}/submissions", submissionH.ListByComp)

			// 比赛管理：创建、查询全部（含未激活）、更新、删除
			r.Post("/competitions", compH.Create)
			r.Get("/competitions", compH.ListAll)
			r.Put("/competitions/{id}", compH.Update)
			r.Delete("/competitions/{id}", compH.Delete)

			// 比赛题目分配：添加/移除题目
			r.Post("/competitions/{id}/challenges", compH.AddChallenge)
			r.Delete("/competitions/{id}/challenges/{challenge_id}", compH.RemoveChallenge)
		})
	})

	// 初始化所有插件并注册路由
	plugins := []plugin.Plugin{
		leaderboard.New(),  // 排行榜插件
		notification.New(), // 通知插件
		analytics.New(),    // 分析插件
		topthree.New(),     // 一二三血插件
		hints.New(),        // 题目提示插件
	}
	for _, p := range plugins {
		p.Register(r, st.DB(), auth)
	}

	// 启动 HTTP 服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
