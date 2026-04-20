# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 命令

```bash
# 构建
go build ./...

# 运行服务器（从项目根目录）
go run ./cmd/server -config config.yaml

# 生成测试数据（15个比赛、50个题目、每个比赛30个用户）
go run ./cmd/seed/
TEST_DSN="root:pass@tcp(host:3306)/ctf?parseTime=true" go run ./cmd/seed/

# 测试脚本（每个资源领域独立测试）
./scripts/test-competitions.sh   # 比赛CRUD + 开始/结束
./scripts/test-challenges.sh     # 题目CRUD
./scripts/test-submissions.sh    # Flag提交 + 记录
./scripts/test-leaderboard.sh    # 排行榜
./scripts/test-notifications.sh  # 通知
./scripts/test-hints.sh          # 提示CRUD
./scripts/test-analytics.sh      # 分析

# 一键跑全部测试脚本
./scripts/demo.sh
./scripts/demo.sh competitions submissions  # 只跑指定模块

# 环境变量
BASE_URL=http://host:8080 JWT_SECRET=xxx ./scripts/test-leaderboard.sh

# 运行所有测试
go test ./...

# 仅运行集成测试（需要 MySQL）
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
- `internal/config/` — YAML 配置（`server.port`、`db.*`、`jwt.secret`、`jwt.admin_role`、`log.*`、`ratelimit.*`）
- `internal/uuid/` — UUID v4 生成器（32字符十六进制，无连字符）
- `internal/logger/` — 基于 `log/slog` 的双输出日志（stdout + 可选文件），支持级别配置
- `internal/auth/` - 一个简单的auth模块，别的模块只能通过http接口和此模块对接

**中间件：**
- `internal/middleware/auth.go` — JWT 认证（`Authenticate`）和管理员关卡（`RequireAdmin`）。从 claims 提取 `sub`→`user_id` 和 `role` 到 context
- `internal/middleware/ratelimit.go` — 基于 `httprate` 的限流，支持按 IP 和按用户 ID，用 `r.With(...)` 内联中间件应用到提交路由

**插件系统：**
- `internal/plugin/` — `Plugin` 接口：`Name() string` 和 `Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]Plugin)`。插件通过名称标识，支持依赖注入
- `internal/plugin/names.go` — 插件名称常量：`NameLeaderboard`、`NameNotification`、`NameHints`、`NameAnalytics`、`NameTopThree`
- `internal/pluginutil/` — 插件共享工具：`WriteJSON`/`WriteError` 响应、`ParseID` 验证、`DBTX` 接口、共享查询函数（`GetCompChallenges`、`GetCorrectSubmissions`、`GetUserScores` 等）
- `plugins/topthree/` — 一血追踪，**事件驱动**：订阅 `EventCorrectSubmission`，实现 `TopThreeProvider` 接口暴露给其他插件
- `plugins/leaderboard/` — 每个比赛的排行榜，通过 `TopThreeProvider` 接口获取三血数据，不直接查询 topthree_records 表
- `plugins/notification/` — 每个比赛的通知（管理员创建，所有用户查看）
- `plugins/hints/` — 题目提示（管理员管理，用户查看可见提示）
- `plugins/analytics/` — 比赛分析，所有查询通过 `pluginutil` 共享函数，无直接 SQL

**事件系统：**
- `internal/event/` — 进程内 pub/sub，基于 `sync.RWMutex`。`Publish` 在独立 goroutine 中调用订阅者，不阻塞发布者。当前事件类型：`EventCorrectSubmission`

## 关键设计决策

- **UUID res_id**：所有实体使用 `res_id VARCHAR(32)` 作为公开 ID。自增 `id` 列 `json:"-"` 仅内部使用。API 路径和响应仅使用 `res_id`
- **BaseModel**：所有 model 嵌入 `BaseModel`（`id`、`res_id`、`created_at`、`updated_at`、`is_deleted`）。所有查询包含 `WHERE is_deleted = 0` 软删除过滤
- **单一store**：`*store.Store` 实现所有 store 接口，传递给所有 service
- **比赛范围**：没有全局排行榜或通知。所有内容限定在比赛范围内。向后兼容支持比赛外提交
- **插件数据库访问限制**：插件不允许直接查询主表，只能查询自己的表。主表查询必须通过 `pluginutil` 共享函数。插件间通信通过接口（如 `TopThreeProvider`）
- **无外键**：数据库不使用外键约束
- **系统日志**：关键操作必须有操作日志, 特别是错误日志，还有flag提交日志
- **接口限流**：提交端点有按用户 ID 的限流（未认证时回退到 IP 限流）

## 集成测试

`internal/integration/` 连接真实 MySQL。每个测试调用 `cleanup(t)` 按依赖顺序清除数据。`testDSN` 从 `TEST_DSN` 环境变量读取，`testSecret` 与生产密钥分开。

## 约束
- **auth模块**: 此模块可以单独出来，可以没有此模块，本项目可以对接其他auth模块
- **新功能**: 每次添加新功能，都必须添加完整的测试用例
- **修改bug**: 改一个 bug，要写一个测试用例，保证此 bug 不会再次发生
- **Model基类**: 所有 model 都要基于 BaseModel
- **函数参数**: 所有函数参数不能多于4个，采用结构体封装
- **输入验证**: 字符串字段有长度限制（title/flag 最多255字符，description 最多4096字符）
- **查询数据库**: 插件不允许直接查询数据库，仅可以查自己插件的数据库, 不能写 SQL 查主表，必须用 pluginutil 中的共享查询函数
