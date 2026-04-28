# anatomy.md

> Auto-maintained by OpenWolf. Last scanned: 2026-04-28T01:51:14.312Z
> Files: 128 tracked | Anatomy hits: 0 | Misses: 0

## ./

- `.gitignore` — Git ignore rules (~32 tok)
- `CLAUDE.md` — OpenWolf (~1599 tok)
- `config.yaml` (~70 tok)
- `config.yaml.example` (~92 tok)
- `go.mod` — Go module definition (~113 tok)
- `go.sum` — Go dependency checksums (~585 tok)
- `openapi.yaml` (~13750 tok)
- `README.md` — Project documentation (~1535 tok)
- `worktree.md` — Superpowers 中 Git Worktrees 的使用详解 (~447 tok)

## .claude/

- `settings.json` (~736 tok)
- `settings.local.json` (~2017 tok)

## .claude/rules/

- `openwolf.md` (~313 tok)

## cmd/auth-server/

- `config.yaml` (~62 tok)
- `main.go` — 是认证服务的独立 HTTP 服务器入口。 (~565 tok)

## cmd/seed/

- `main.go` — 是测试数据填充工具的入口。 (~5796 tok)

## cmd/server/

- `main.go` — 是 CTF 比赛平台的 HTTP 服务器入口。 (~797 tok)

## docs/CODEMAPS/

- `architecture.md` — CTF Platform Architecture (~527 tok)
- `backend.md` — Backend Architecture (~958 tok)
- `data.md` — Database Schema (~583 tok)
- `dependencies.md` — Dependencies (~334 tok)

## docs/superpowers/plans/

- `2026-04-17-ctf-platform.md` — CTF 解题赛系统 - 实现计划 (~5551 tok)
- `2026-04-17-dashboard-plugin.md` — Dashboard Plugin Implementation Plan (~5925 tok)
- `2026-04-17-hints-plugin.md` — Hints Plugin Implementation Plan (~3241 tok)
- `2026-04-18-dashboard-topthree-refactor.md` — Dashboard → TopThree 依赖重构 实现计划 (~4924 tok)
- `2026-04-18-rate-limit-implementation.md` — Rate Limiting Implementation Plan (~3063 tok)
- `2026-04-18-router-refactor.md` — Router Refactor Implementation Plan (~1757 tok)
- `2026-04-18-topthree-plugin.md` — Top Three Plugin Implementation Plan (~4672 tok)
- `2026-04-19-competition-start-end.md` — Competition Start/End Implementation Plan (~3381 tok)
- `2026-04-19-system-logging.md` — 系统日志 Implementation Plan (~3827 tok)
- `2026-04-20-plugin-db-access-restriction-plan.md` — 插件数据库访问限制 Implementation Plan (~4801 tok)
- `2026-04-20-seed-http.md` — Seed HTTP Migration Implementation Plan (~4094 tok)
- `2026-04-20-soft-delete-plan.md` — 软删除改造 Implementation Plan (~1495 tok)
- `2026-04-20-standalone-auth-server.md` — Standalone Auth Server Implementation Plan (~4466 tok)
- `2026-04-20-test-distribution.md` — 测试分布实现计划 (~2332 tok)
- `2026-04-27-team-competition-mode.md` — Team Competition Mode Implementation Plan (~15043 tok)

## docs/superpowers/specs/

- `2026-04-17-ctf-platform-design.md` — CTF 解题赛系统 - 后端设计 (~1253 tok)
- `2026-04-17-dashboard-plugin-design.md` — Dashboard 插件设计文档 (~2076 tok)
- `2026-04-17-hints-plugin-design.md` — Hints Plugin Design (~938 tok)
- `2026-04-18-dashboard-topthree-refactor-design.md` — Dashboard → TopThree 依赖重构设计 (~920 tok)
- `2026-04-18-extract-shared-helpers-design.md` — 重构：提取共享辅助函数 + Store 层统一 (~1354 tok)
- `2026-04-18-rate-limit-design.md` — Rate Limiting 设计文档 (~676 tok)
- `2026-04-18-router-refactor-design.md` — Router 重构设计文档 (~831 tok)
- `2026-04-18-topthree-plugin-design.md` — Top Three 插件设计文档 (~1204 tok)
- `2026-04-19-competition-start-end-design.md` — 比赛开始/结束操作设计 (~572 tok)
- `2026-04-19-demo-scripts-split-design.md` — Demo Scripts Split Design (~1171 tok)
- `2026-04-19-system-logging-design.md` — 系统日志设计 (~954 tok)
- `2026-04-20-auth-team-management-design.md` — Auth & Team Management Design (~994 tok)
- `2026-04-20-plugin-db-access-restriction-design.md` — 插件数据库访问限制设计文档 (~1598 tok)
- `2026-04-20-seed-http-design.md` — Seed HTTP Migration Design (~760 tok)
- `2026-04-20-soft-delete-design.md` — 软删除改造设计文档 (~1595 tok)
- `2026-04-20-standalone-auth-server-design.md` — Standalone Auth Server Design (~995 tok)
- `2026-04-20-test-distribution-design.md` — 测试分布设计 (~907 tok)
- `2026-04-21-user-team-relationship-design.md` — User-Team Relationship Redesign (~1657 tok)
- `2026-04-27-team-competition-mode-design.md` — Team Competition Mode Design (~2487 tok)

## internal/auth/

- `auth_test.go` — mockUserStore (37 fields); methods: CreateUser, GetUserByUsername, GetUserByID, ListUsersByTeam (~1337 tok)
- `handler.go` — HTTP handlers: authWriteJSON, authWriteError (~669 tok)
- `model.go` — 实现用户注册登录和队伍管理。 (~649 tok)
- `mysql.go` — AuthStore (92 fields); methods: CreateUser, GetUserByUsername, GetUserByID, ListUsersByTeam (~2686 tok)
- `router.go` — RouteDeps (0 fields) (~368 tok)
- `service.go` — AuthService (34 fields); methods: Register, Login, GenerateToken, VerifyToken (~1025 tok)
- `store.go` — UserStore 定义用户相关的数据访问接口。 (~619 tok)
- `team_handler.go` — TeamHandler (46 fields); methods: List, Get, Create, Update (~2452 tok)
- `team_service_test.go` — mockTeamStore (58 fields); methods: CreateTeam, GetTeamByID, ListTeams, UpdateTeam (~2085 tok)
- `team_service.go` — TeamService (104 fields); methods: CreateTeam, CreateTeamWithCreator, GetTeam, ListTeams (~2024 tok)
- `verify_handler_test.go` — TestVerify_ValidToken, TestVerify_MissingToken, TestVerify_InvalidToken, TestVerify_ExpiredToken (~641 tok)
- `verify_handler.go` — VerifyHandler (1 fields); methods: Verify (~268 tok)

## internal/config/

- `config.go` — 提供 YAML 配置文件的加载与解析功能。 (~830 tok)

## internal/event/

- `event.go` — 实现简单的进程内事件发布/订阅机制。 (~417 tok)

## internal/handler/

- `challenge.go` — 实现 HTTP 请求处理层（题目相关）。 (~1359 tok)
- `competition.go` — 实现比赛相关的 HTTP 请求处理。 (~3673 tok)
- `submission.go` — 实现 Flag 提交相关的 HTTP 请求处理。 (~706 tok)
- `util.go` — 提供 HTTP 响应工具函数和输入验证辅助。 (~361 tok)

## internal/integration/

- `integration_test.go` — TestMain, TestListChallenges, TestGetChallenge, TestAdminCreateChallenge + 5 more (~13589 tok)

## internal/logger/

- `logger_test.go` — TestInitStdoutOnly, TestInitWithFile, TestParseLevel, TestLogFileContent, TestLevelFiltering (~635 tok)
- `logger.go` — 提供统一的结构化日志功能。 (~743 tok)

## internal/middleware/

- `auth.go` — 提供 HTTP 中间件，包括 JWT 认证和管理员权限校验。 (~925 tok)
- `ratelimit_test.go` — TestLimitByIP, TestLimitByUserID, TestLimitByUserID_FallbackToIP (~883 tok)
- `ratelimit.go` — 提供 HTTP 中间件，包括频率限制。 (~343 tok)
- `validate.go` — 提供 HTTP 中间件。 (~695 tok)

## internal/model/

- `competition.go` — Competition (52 fields); methods: Validate, Validate, Validate (~842 tok)
- `constants.go` — Boolean constants 用于数据库中 TINYINT(1) 类型的布尔值字段。 (~52 tok)
- `model.go` — 定义了 CTF 比赛平台的领域模型（实体结构体）。 (~1885 tok)

## internal/plugin/

- `names.go` (~54 tok)
- `plugin.go` — 定义插件系统的核心接口。 (~170 tok)

## internal/pluginutil/

- `http.go` — HTTP handlers: WriteJSON, WriteError (~165 tok)
- `pluginutil_test.go` — TestParseID, TestWriteJSON, TestWriteError (~470 tok)
- `queries.go` — 提供插件共享的数据库查询函数。 (~5297 tok)
- `validate.go` — ParseID 校验 res_id 是否为有效的 32 字符十六进制字符串。 (~64 tok)

## internal/router/

- `api.go` — 负责 API 路由的集中注册。 (~301 tok)
- `challenges.go` — RegisterChallengeRoutes (~236 tok)
- `competitions.go` — RegisterCompetitionRoutes (~668 tok)
- `submissions.go` — RegisterSubmissionRoutes (~170 tok)

## internal/service/

- `challenge.go` — 实现业务逻辑层。 (~997 tok)
- `competition.go` — 实现比赛相关的业务逻辑。 (~2459 tok)
- `submission.go` — 实现 Flag 提交相关的业务逻辑。 (~1467 tok)
- `team_resolver.go` — TeamResolver (9 fields); methods: GetUserTeam (~326 tok)

## internal/store/

- `mysql.go` — Store (90 fields); methods: Close, DB, ListEnabled, GetEnabledByID (~4267 tok)
- `store.go` — 定义数据访问层的接口。 (~1066 tok)

## internal/testutil/

- `testutil.go` — provides shared integration test infrastructure for the ad7 project. (~2927 tok)

## internal/uuid/

- `uuid.go` — 提供 UUID v4 生成器和验证器。 (~361 tok)

## plugins/analytics/

- `analytics_test.go` — TestMain, TestAnalyticsOverview, TestAnalyticsCategories, TestAnalyticsUsers, TestAnalyticsChallenges (~1427 tok)
- `analytics.go` — 实现比赛分析插件。 (~4182 tok)

## plugins/hints/

- `hints_test.go` — TestMain, TestHintsCRUD (~958 tok)
- `hints.go` — 实现题目提示插件。 (~1730 tok)

## plugins/leaderboard/

- `leaderboard_test.go` — TestMain, TestCompetitionLeaderboard (~1003 tok)
- `leaderboard.go` — 实现比赛排行榜插件。 (~1789 tok)

## plugins/notification/

- `notification_test.go` — TestMain, TestNotificationCRUD (~1202 tok)
- `notification.go` — 实现比赛通知插件。 (~1867 tok)

## plugins/topthree/

- `api.go` (~668 tok)
- `model.go` — topThreeRecord (14 fields) (~385 tok)
- `provider.go` — TopThreeProvider 定义 topthree 插件暴露给其他插件的接口 (~170 tok)
- `schema.sql` — Database schema (~242 tok)
- `topthree_test.go` — TestMain, TestTopThreeEventDriven, TestTopThreeAuth, TestTopThreeDuplicateUser (~1881 tok)
- `topthree.go` — 实现三血（前三名正确提交者）追踪插件。 (~2238 tok)

## scripts/

- `demo.sh` — demo.sh — Run all API test scripts (~317 tok)
- `full-test.sh` — full-test.sh — 完整的端到端测试脚本 (~2062 tok)
- `quick-seed.sh` — quick-seed.sh — 快速生成测试数据（直接操作数据库） (~878 tok)
- `test-analytics.sh` — test-analytics.sh — Interactive menu for Analytics API (~751 tok)
- `test-challenges.sh` — test-challenges.sh — Interactive menu for Challenge API (~956 tok)
- `test-competitions.sh` — test-competitions.sh — Interactive menu for Competition API (~1260 tok)
- `test-hints.sh` — test-hints.sh — Interactive menu for Hints API (~946 tok)
- `test-leaderboard.sh` — test-leaderboard.sh — Interactive menu for Leaderboard API (~570 tok)
- `test-notifications.sh` — test-notifications.sh — Interactive menu for Notification API (~705 tok)
- `test-submissions.sh` — test-submissions.sh — Interactive menu for Submission API (~1148 tok)
- `test-topthree-leaderboard.sh` — test-topthree-leaderboard.sh — 测试 flag 提交、一二三血和排行榜 (~964 tok)

## sql/

- `schema.sql` — Database schema (~1684 tok)

## sql/migrations/

- `001_team_members.sql` — User-Team Relationship Redesign Migration (~472 tok)
- `002_team_competition_mode.sql` — Competition mode and team join mode (~315 tok)
