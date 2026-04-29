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
	"ad7/internal/logger"
	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/pluginutil"
	"ad7/internal/router"
	"ad7/internal/service"
	"ad7/internal/store"
	"ad7/plugins/analytics"
	"ad7/plugins/cache"
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
// 5. 初始化 Service 层（题目、提交、比赛、认证）
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

	// 初始化日志系统（stdout + 可选文件输出）
	if err := logger.Init(cfg.Log); err != nil {
		log.Fatalf("init logger: %v", err)
	}

	// 通过 DSN 连接 MySQL 数据库
	st, err := store.New(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer st.Close()

	// 创建认证中间件，传入认证服务地址和管理员角色名
	authMW := middleware.NewAuth(cfg.Auth.URL, cfg.JWT.AdminRole)

	// 创建 TeamResolver，用于解析用户队伍
	teamResolver := service.NewTeamResolver(cfg.Auth.URL)

	// 初始化 Service 层，注入对应的 Store 接口
	challengeSvc := service.NewChallengeService(st)
	submissionSvc := service.NewSubmissionService(st, st, st, teamResolver)
	compSvc := service.NewCompetitionService(st)

	// 初始化所有插件（注意顺序：缓存插件最先加载）
	plugins := []plugin.Plugin{
		cache.New(),         // 缓存插件（最先加载，供其他插件使用）
		leaderboard.New(),  // 排行榜插件
		notification.New(), // 通知插件
		analytics.New(),    // 分析插件
		topthree.New(),     // 一二三血插件
		hints.New(),        // 题目提示插件
	}

	// 构建插件名称到实例的映射，用于依赖注入
	pluginMap := make(map[string]plugin.Plugin)
	for _, p := range plugins {
		pluginMap[p.Name()] = p
	}

	// 获取缓存提供器（供 handler 使用）
	var cacheProvider pluginutil.CacheProvider
	if cp, ok := pluginMap[plugin.NameCache].(cache.Provider); ok {
		cacheProvider = cp
	}

	// 初始化 Handler 层
	challengeH := handler.NewChallengeHandler(challengeSvc)
	submissionH := handler.NewSubmissionHandler(submissionSvc)
	compH := handler.NewCompetitionHandler(compSvc, teamResolver, cacheProvider)

	// 创建 chi 路由器并挂载全局中间件
	r := chi.NewRouter()
	r.Use(chimw.Logger)    // 请求日志记录
	r.Use(chimw.Recoverer) // panic 恢复，防止服务崩溃
	r.Use(middleware.MaxBodySize(1 << 20)) // 1MB body 限制

	// 注册 API v1 路由组（通过 router 包统一注册，需要 JWT）
	router.RegisterAPIV1(r, router.RouteDeps{
		Auth:         authMW,
		Config:       cfg,
		ChallengeH:  challengeH,
		CompetitionH: compH,
		SubmissionH:  submissionH,
	})

	// 注册所有插件路由，传递依赖映射
	for _, p := range plugins {
		p.Register(r, st.DB(), authMW, pluginMap)
	}

	// 确保缓存插件在关闭时停止
	if cp, ok := pluginMap[plugin.NameCache].(*cache.Plugin); ok {
		defer cp.Stop()
	}

	// 启动 HTTP 服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	logger.Info("server starting", "port", cfg.Server.Port)
	log.Fatal(http.ListenAndServe(addr, r))
}
