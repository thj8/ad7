# OpenWolf

@.wolf/OPENWOLF.md

This project uses OpenWolf for context management. Read and follow .wolf/OPENWOLF.md every session. Check .wolf/cerebrum.md before generating code. Check .wolf/anatomy.md before reading files.


# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 命令

```bash
# 构建
go build ./...

# 运行 CTF 服务器（从项目根目录）
go run ./cmd/server -config config.yaml

# 运行认证服务器（从项目根目录）
go run ./cmd/auth-server -config cmd/auth-server/config.yaml

# 生成测试数据（15个比赛、50个题目、每个比赛30个用户）
go run ./cmd/seed/
TEST_DSN="root:pass@tcp(host:3306)/ctf?parseTime=true" go run ./cmd/seed/

# 运行所有测试
go test ./...

# 仅运行集成测试（需要 MySQL）
souce .env
go test ./internal/integration/... -v -count=1

# 运行单个测试
go test ./internal/integration/... -v -run TestSubmitFlag -count=1

# 应用数据库架构
mysql -h <host> -u root -p<password> ctf < sql/schema.sql
```

## 架构

分层 Go 服务：**router → handler → service → store** + **插件系统** + **事件系统**

**核心层：**
- `cmd/server/main.go` — 按顺序组装：config → logger → store → middleware → services → handlers → router → plugins → HTTP server
- `internal/router/` — 路由注册，使用 `RouteDeps` 结构体封装所有依赖。`api.go` 创建 `/api/v1` 路由组，按领域拆分为 `challenges.go`、`competitions.go`、`submissions.go`
- `internal/model/` — 领域结构体，全部嵌入 `BaseModel`（`id`、`res_id`、`created_at`、`updated_at`、`is_deleted`）。`Flag` 字段 `json:"-"` 永不暴露
- `internal/store/` — `store.go` 定义接口；`mysql.go` 单个 `*Store` 实现所有接口
- `internal/service/` — 业务逻辑层。`SubmissionService.SubmitInComp` 在正确提交时发布 `EventCorrectSubmission`
- `internal/handler/` — HTTP 层，使用单独的请求结构体接收 `json:"-"` 的字段
- `internal/config/` — YAML 配置（`server.port`、`db.*`、`auth.url`、`jwt.secret`、`jwt.admin_role`、`log.*`、`ratelimit.*`、`cache.*`）
- `internal/cache/` — 泛型内存缓存，支持 TTL 过期、懒淘汰、后台清理、`GetOrSet` 封装函数调用
- `internal/uuid/` — UUID v4 生成器（32字符十六进制，无连字符）
- `internal/logger/` — 基于 `log/slog` 的双输出日志（stdout + 可选文件），支持级别配置
- `internal/auth/` — 认证模块：用户注册/登录、JWT token 签发和验证、队伍 CRUD + 成员管理。独立运行在 `cmd/auth-server/` 中，通过 HTTP 接口提供服务。使用 `team_members` 关联表管理用户-队伍关系，支持 `captain`/`member` 角色
- `cmd/auth-server/` — 独立认证服务器入口，端口 8081。提供 `/api/v1/register`、`/api/v1/login`、`/api/v1/verify`、`/api/v1/teams/*` 等端点。新增端点：`PUT /api/v1/admin/teams/{id}/captain`、`POST /api/v1/admin/teams/{id}/transfer-captain`

**中间件：**
- `internal/middleware/auth.go` — 认证中间件（`Authenticate`）通过 HTTP 调用 auth 服务的 `POST /api/v1/verify` 验证 JWT token，从响应提取 `user_id` 和 `role` 注入 context。`RequireAdmin` 检查 context 中的 role。`NewAuth(authURL, adminRole)` 接收 auth 服务地址而非 JWT secret
- `internal/middleware/ratelimit.go` — 基于 `httprate` 的限流，支持按 IP 和按用户 ID，用 `r.With(...)` 内联中间件应用到提交路由

**插件系统：**
- `internal/plugin/` — `Plugin` 接口：`Name() string` 和 `Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]Plugin)`。插件通过名称标识，支持依赖注入
- `internal/plugin/names.go` — 插件名称常量：`NameLeaderboard`、`NameNotification`、`NameHints`、`NameAnalytics`、`NameTopThree`、`NameCache`
- `internal/pluginutil/` — 插件共享工具：`WriteJSON`/`WriteError` 响应、`ParseID` 验证、`DBTX` 接口、共享查询函数（`GetCompChallenges`、`GetCorrectSubmissions`、`GetUserScores`、`GetTeamCorrectSubmissions`、`GetTeamScores` 等）
- `plugins/cache/` — 通用缓存插件，**事件驱动**：订阅 `EventCorrectSubmission` 清除相关缓存，实现 `Provider` 接口暴露给其他插件，为 auth 中间件提供 token 缓存，支持懒淘汰和后台清理
- `plugins/topthree/` — 一血追踪，**事件驱动**：订阅 `EventCorrectSubmission`，实现 `TopThreeProvider` 接口暴露给其他插件，支持个人模式和队伍模式，缓存填满后不再失效
- `plugins/leaderboard/` — 每个比赛的排行榜，通过 `TopThreeProvider` 接口获取三血数据，不直接查询 topthree_records 表，支持个人模式和队伍模式切换，支持缓存
- `plugins/notification/` — 每个比赛的通知（管理员创建，所有用户查看）
- `plugins/hints/` — 题目提示（管理员管理，用户查看可见提示）
- `plugins/analytics/` — 比赛分析，所有查询通过 `pluginutil` 共享函数，无直接 SQL，支持个人模式和队伍模式的统计数据，支持缓存

**比赛模式：**
- 支持两种比赛模式：`individual`（个人模式，默认）和 `team`（队伍模式）
- 支持两种队伍加入方式：`free`（自由加入，任何有队伍的用户可进入）和 `admin`（管理模式，仅管理员添加的队伍可进入）
- `competitions.mode` 字段指定模式，`competitions.team_join_mode` 指定队伍加入方式
- `competition_teams` 表管理比赛-队伍关联
- 队伍模式下，单个队伍成员的正确提交计入整个队伍，题目级别去重（一个队伍对一道题目只能提交一次正确答案）
- 事件 `EventCorrectSubmission` 包含 `TeamID` 字段（队伍模式时有值）

**事件系统：**
- `internal/event/` — 进程内 pub/sub，基于 `sync.RWMutex`。`Publish` 在独立 goroutine 中调用订阅者，不阻塞发布者。当前事件类型：`EventCorrectSubmission`

## 文档
**API接口** ：@openapi.yaml


## 关键设计决策

- **UUID res_id**：所有实体使用 `res_id VARCHAR(32)` 作为公开 ID。自增 `id` 列 `json:"-"` 仅内部使用。API 路径和响应仅使用 `res_id`
- **BaseModel**：所有 model 嵌入 `BaseModel`（`id`、`res_id`、`created_at`、`updated_at`、`is_deleted`）。所有查询包含 `WHERE is_deleted = 0` 软删除过滤
- **单一store**：`*store.Store` 实现所有 store 接口，传递给所有 service
- **比赛范围**：没有全局排行榜或通知。所有内容限定在比赛范围内。向后兼容支持比赛外提交
- **插件数据库访问限制**：插件不允许直接查询主表，只能查询自己的表。主表查询必须通过 `pluginutil` 共享函数。插件间通信通过接口（如 `TopThreeProvider`）
- **无外键**：数据库不使用外键约束
- **系统日志**：关键操作必须有操作日志, 特别是错误日志，还有flag提交日志
- **接口限流**：提交端点有按用户 ID 的限流（未认证时回退到 IP 限流）
- **auth 服务独立部署**：`cmd/auth-server/` 是独立的认证服务，CTF 中间件通过 HTTP 调用 `POST /api/v1/verify` 验证 token。verify 端点本身不需要认证（它验证的就是传入的 token）

## 集成测试

`internal/integration/` 连接真实 MySQL。`testutil.NewTestEnv` 启动两个 httptest.Server：一个 auth 服务（模拟认证服务器）和一个 CTF 服务。每个测试调用 `Cleanup(t)` 按依赖顺序清除数据（含 users、teams 表）。`TEST_DSN` 环境变量配置数据库连接。

## 约束
- **避免造轮子**: 写什么功能先查询github有没有开源的模块，有的话直接参考或者直接使用
- **auth模块**: 独立 HTTP 服务（`cmd/auth-server/`），CTF 项目通过 `POST /api/v1/verify` 验证 token。CTF 中间件不解析 JWT，仅通过 HTTP 调用 auth 服务。可替换为其他兼容的 auth 服务
- **双服务架构**: CTF 服务器（端口 8080）和认证服务器（端口 8081）共享同一个 MySQL 数据库，通过 HTTP 通信
- **新功能**: 每次添加新功能，都必须添加完整的测试用例
- **修改bug**: 改一个 bug，要写一个测试用例，保证此 bug 不会再次发生
- **Model基类**: 所有 model 都要基于 BaseModel
- **函数参数**: 所有函数参数不能多于4个，采用结构体封装
- **输入验证**: 字符串字段有长度限制（title/flag 最多255字符，description 最多4096字符）
- **查询数据库**: 插件不允许直接查询数据库，仅可以查自己插件的数据库, 不能写 SQL 查主表，必须用 pluginutil 中的共享查询函数
- **用户-队伍关系**: 使用 `team_members` 关联表替代 `users.team_id` 列，支持多队预备（当前通过服务层约束单队）。支持队伍内部角色：`captain`（队长）和 `member`（成员）。每个队伍有且仅有一个队长，队长在有其他成员时不能被移除，必须先转移队长权限
- **队伍比赛模式**: 比赛创建时可选择个人模式或队伍模式。队伍模式下，排行榜、topthree、analytics 都按队伍统计而非个人。队伍提交会去重，同一题目同一队伍只有首次正确提交计分
- **访问控制**: 队伍模式下，CheckCompAccess 验证用户是否属于有权限参加比赛的队伍（自由加入模式：用户有队伍即可；管理模式：队伍需在 competition_teams 表中）
- **向后兼容**: 所有现有代码默认使用个人模式，mode 字段默认值为 "individual"，确保不影响现有功能
- **包纯净性（严格执行）**: 两个服务（CTF / Auth）共享的包必须只包含双方都需要的代码，不允许混入单方专属逻辑
  - `internal/basemodel/` — 共享基础类型（`BaseModel`、`Time`、`ValidationError`、验证工具函数），双方都可引用
  - `internal/db/` — 共享数据库连接管理（`Connect`），双方都可引用
  - `internal/config/` — 共享配置类型（`ServerConfig`、`DBConfig`、`JWTConfig`、`LogConfig`、`RateLimitRule`）+ CTF 专用 `Load` 函数
  - `internal/middleware/` — 共享中间件（`Authenticate`、`RequireAdmin`、`LimitByIP`、`LimitByUserID`、`MaxBodySize`），不允许包含 CTF 专属逻辑
  - `internal/auth/` — Auth 服务专属，包含自己的 `AuthServerConfig`/`LoadAuthConfig`，引用 `basemodel` 而非 `model`
  - `internal/ctxutil/` — CTF 专属（URL 参数验证、Context 存取），Auth 服务禁止引用
  - `internal/model/` — CTF 专属领域模型（`Challenge`、`Submission` 等），通过 type alias 重导出 `basemodel` 类型保持向后兼容，Auth 服务禁止引用
  - `internal/store/` — CTF 专属（store 接口和 MySQL 实现），Auth 服务禁止引用（用 `internal/db/` 获取连接）
  - **新增代码时**: 如果一个函数/类型只被一个服务使用，必须放到该服务专属的包中，不能放入共享包
