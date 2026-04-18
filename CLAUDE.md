# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 提供使用本仓库代码的指导。

## 命令

```bash
# 构建
go build ./...

# 运行服务器（从项目根目录）
go run ./cmd/server -config config.yaml

# 生成测试数据（15个比赛、50个题目、每个比赛30个用户）
go run ./cmd/seed/
TEST_DSN="root:pass@tcp(host:3306)/ctf?parseTime=true" go run ./cmd/seed/

# 演示脚本（查询比赛、提交Flag、查看排行榜）
./scripts/demo.sh
BASE_URL=http://host:8080 ./scripts/demo.sh

# 运行所有测试
go test ./...

# 仅运行集成测试（需要 MySQL 在 192.168.5.44）
go test ./internal/integration/... -v -count=1

# 运行单个测试
go test ./internal/integration/... -v -run TestSubmitFlag -count=1

# 应用数据库架构
mysql -h 192.168.5.44 -u root -pasfdsfedarjeiowvgfsd ctf < sql/schema.sql
```

## 架构

分层 Go 服务：**handler → service → store** + **插件系统**

- `cmd/server/main.go` — 组装所有组件：加载配置、打开数据库、创建 store/service/handler、注册 chi 路由、加载插件、启动 HTTP 服务器
- `cmd/seed/main.go` — 填充测试数据：50个题目、15个比赛、每个比赛30个用户，解题率差异化（顶部用户72%）
- `internal/config/` — YAML 配置加载（`server.port`、`db.*`、`jwt.secret`、`jwt.admin_role`）
- `internal/model/` — 领域结构体：`Challenge`、`Submission`、`Notification`、`Competition`、`CompetitionChallenge`。`Flag` 字段有 `json:"-"` 所以永远不会出现在 API 响应中。所有实体使用 UUID `res_id`（32字符十六进制字符串，无连字符）作为公开 ID。
- `internal/store/` — `store.go` 定义 `ChallengeStore`、`SubmissionStore`、`CompetitionStore` 接口；`mysql.go` 在单个 `*Store` 结构体上实现所有接口
- `internal/service/` — 业务逻辑：`ChallengeService`（CRUD）、`SubmissionService`（Flag验证、比赛内提交）、`CompetitionService`（比赛CRUD、题目分配）
- `internal/handler/` — 仅 HTTP 层；使用单独的请求结构体来接收模型上有 `json:"-"` 的字段
- `internal/middleware/` — JWT 认证（`Authenticate`）和管理员关卡（`RequireAdmin`）；从 claims 中提取 `sub`→`user_id` 和 `role` 到上下文
- `internal/plugin/` — `Plugin` 接口：`Register(r chi.Router, db *sql.DB, auth *middleware.Auth)`
- `internal/snowflake/` — UUID v4 生成器（32字符十六进制字符串，无连字符）
- `plugins/leaderboard/` — 每个比赛的排行榜，按总分降序排列，同分按最早解题时间升序排列
- `plugins/notification/` — 每个比赛的通知，管理员创建，所有用户可查看
- `plugins/hints/` — 题目提示系统，管理员管理，用户查看可见提示
- `plugins/dashboard/` — 比赛仪表盘，含一血追踪
- `plugins/analytics/` — 比赛分析（概览、分类、用户、题目）

## 关键设计决策

**UUID res_id**：所有实体使用 `res_id VARCHAR(32)`（UUID v4，32字符十六进制字符串无连字符）作为公开 ID。自增 `id` 列仅内部使用（`json:"-"`）。API 路径和响应仅使用 `res_id`。

**Flag 字段**：`model.Challenge.Flag` 是 `json:"-"`。Handler 使用单独的请求结构体（`createRequest`、`updateRequest`）从传入的 JSON 解码 flag，然后在传递给 service 之前手动赋值给模型。

**单一 store 结构体**：`*store.Store` 实现所有 store 接口。在 `main.go` 中传递给所有 service。

**管理员认证**：JWT 中间件读取 `role` claim；`RequireAdmin` 将其与 `cfg.JWT.AdminRole`（默认 `"admin"`）比较。用户身份来自 `sub` claim。

**比赛范围**：没有全局排行榜或全局通知。所有内容都限定在比赛范围内。为了向后兼容，仍支持比赛外的提交。

**插件系统**：插件实现 `Plugin` 接口并在 `main.go` 中注册自己的 chi 路由。它们接收 `*sql.DB` 和 `*middleware.Auth` 用于直接数据库访问和路由保护。

**无外键**：根据项目约束，数据库不使用外键约束。

**输入验证**：字符串字段有长度限制（title/flag 最多255字符，description 最多4096字符）。`parseID` 在无效输入时返回 400。通知 `message` 是必填的。

## 集成测试

`internal/integration/` 中的测试连接到真实的 MySQL 实例。16个测试覆盖：
- 题目 CRUD（列表、获取、提交、管理员CRUD、提交列表）
- 比赛 CRUD
- 比赛题目分配
- 比赛内 Flag 提交
- 每个比赛的排行榜
- 每个比赛的通知

`testDSN` 从 `TEST_DSN` 环境变量读取，回退到硬编码默认值。`testSecret` 与生产密钥分开。

每个测试调用 `cleanup(t)`，按依赖顺序删除所有表中的数据。

## 约束
- 每次添加新功能，都必须添加完整的测试用例
- 改一个bug，要写一个测试用例，保证此bug不会再次发生
- 数据库不要使用外键关联
