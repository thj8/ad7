# AD7 - CTF Jeopardy 平台

一个使用 Go 构建的多比赛 CTF（夺旗赛）Jeopardy 平台。

## 功能特性

- **多比赛支持** — 创建和管理多个独立的比赛，每个比赛有自己的题目集合
- **题目管理** — 管理员 CRUD 操作，使用静态 Flag 验证
- **比赛内 Flag 提交** — 用户在比赛上下文中提交 Flag，防止重复解题
- **每个比赛的排行榜** — 按总分降序排列，同分按最早解题时间打破平局
- **每个比赛的通知** — 管理员可以发布限定在比赛范围内的公告
- **比赛分析** — 详细统计数据，包括概览、分类、用户和题目分析
- **插件系统** — 可扩展的编译时插件接口，用于添加新功能
- **UUID ID** — 所有公开 ID 使用 UUID v4（32字符十六进制字符串无连字符）作为唯一标识符
- **JWT 认证** — Bearer Token 认证，带管理员角色关卡（用户管理在外部处理）

## 快速开始

```bash
# 安装依赖
go mod download

# 配置
cp config.yaml.example config.yaml
cp cmd/auth-server/config.yaml.example cmd/auth-server/config.yaml
# 编辑两个 config.yaml，填入你的 MySQL 和 JWT 设置

# 应用数据库架构
mysql -u root -p your_db < sql/schema.sql

# 生成测试数据（可选）
go run ./cmd/seed/

# 运行服务器（必须按顺序启动）
# 1. 先启动认证服务器（端口 8081）
go run ./cmd/auth-server -config cmd/auth-server/config.yaml

# 2. 再启动 CTF 服务器（端口 8080）
go run ./cmd/server -config config.yaml

# 尝试测试脚本
./scripts/test-competitions.sh   # 比赛CRUD + 开始/结束
./scripts/test-challenges.sh     # 题目CRUD
./scripts/test-submissions.sh    # Flag提交
./scripts/test-leaderboard.sh    # 排行榜
./scripts/test-notifications.sh  # 通知
./scripts/test-hints.sh          # 提示
./scripts/test-analytics.sh      # 分析

# 或一键跑全部
./scripts/demo.sh
```

## 架构说明

### 双服务架构

项目使用独立的认证服务器和 CTF 服务器：

```
认证服务器 (端口 8081)
├── /api/v1/register    # 用户注册
├── /api/v1/login       # 用户登录
├── /api/v1/verify      # Token 验证（供 CTF 服务器调用）
└── /api/v1/teams/*     # 队伍管理

CTF 服务器 (端口 8080)
├── 所有 CTF 业务 API
└── 通过 POST /api/v1/verify 调用认证服务器验证 Token
```

认证服务器和 CTF 服务器共享同一个 MySQL 数据库。

## API 概览

### 认证

所有端点都需要在 `Authorization` 头中提供 Bearer JWT Token。管理员端点另外需要 token claims 中有 `role: admin`。

### 题目（管理员）

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| POST | `/api/v1/admin/challenges` | 创建题目 |
| PUT | `/api/v1/admin/challenges/{id}` | 更新题目 |
| DELETE | `/api/v1/admin/challenges/{id}` | 删除题目 |

### 题目（用户）

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/api/v1/challenges` | 列出已启用的题目 |
| GET | `/api/v1/challenges/{id}` | 获取题目详情 |

### 提交

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| POST | `/api/v1/challenges/{id}/submit` | 提交 Flag（全局） |
| POST | `/api/v1/competitions/{comp_id}/challenges/{id}/submit` | 提交 Flag（比赛内） |
| GET | `/api/v1/admin/competitions/{id}/submissions` | 列出比赛提交（管理员） |

### 比赛（管理员）

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| POST | `/api/v1/admin/competitions` | 创建比赛 |
| PUT | `/api/v1/admin/competitions/{id}` | 更新比赛 |
| DELETE | `/api/v1/admin/competitions/{id}` | 删除比赛 |
| GET | `/api/v1/admin/competitions` | 列出所有比赛 |
| POST | `/api/v1/admin/competitions/{id}/challenges` | 向比赛添加题目 |
| DELETE | `/api/v1/admin/competitions/{id}/challenges/{challenge_id}` | 从比赛移除题目 |
| POST | `/api/v1/admin/competitions/{id}/start` | 开始比赛 |
| POST | `/api/v1/admin/competitions/{id}/end` | 结束比赛 |

### 比赛（用户）

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/api/v1/competitions` | 列出活跃比赛 |
| GET | `/api/v1/competitions/{id}` | 获取比赛详情 |
| GET | `/api/v1/competitions/{id}/challenges` | 列出比赛题目 |

### 插件

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/api/v1/competitions/{id}/leaderboard` | 比赛排行榜 |
| POST | `/api/v1/admin/competitions/{id}/notifications` | 创建比赛通知 |
| GET | `/api/v1/competitions/{id}/notifications` | 列出比赛通知 |
| GET | `/api/v1/competitions/{id}/analytics/overview` | 比赛概览统计 |
| GET | `/api/v1/competitions/{id}/analytics/categories` | 分类统计 |
| GET | `/api/v1/competitions/{id}/analytics/users` | 用户表现统计 |
| GET | `/api/v1/competitions/{id}/analytics/challenges` | 题目难度统计 |
| POST | `/api/v1/admin/challenges/{id}/hints` | 创建题目提示 |
| PUT | `/api/v1/admin/hints/{id}` | 更新题目提示 |
| DELETE | `/api/v1/admin/hints/{id}` | 删除题目提示 |
| GET | `/api/v1/challenges/{id}/hints` | 列出可见的题目提示 |

### 比赛分析

分析插件为比赛提供详细统计数据：

**概览（`/analytics/overview`）**
- 总用户数、题目数、提交数
- 正确提交数量
- 平均每人解题数
- 平均解题时间（从比赛开始计算）
- 完成率（平均每人解题百分比）

**按分类（`/analytics/categories`）**
- 每个分类的总题目数
- 每个分类的总解题数
- 每个分类的独立解题用户数
- 平均每人解题数
- 成功率（正确 / 总提交数）

**用户统计（`/analytics/users`）**
- 每个用户的总解题数、总分、总尝试数
- 每个用户的成功率
- 第一次和最后一次解题时间
- 按总分降序、第一次解题时间升序排列

**题目统计（`/analytics/challenges`）**
- 每个题目的总解题数、尝试数、成功率
- 独立解题用户数
- 第一次解题时间
- 平均解题时间（从第一次提交到正确提交）

## 技术栈

- **Go 1.22** 使用 [chi](https://github.com/go-chi/chi/v5) 路由器
- **MySQL** 通过 `database/sql`
- **JWT**（HS256）通过 `golang-jwt/jwt/v5`
- **UUID v4** — 自定义实现，生成 32 字符十六进制 UUID 无连字符

## 项目结构

```
.
├── cmd/
│   ├── server/           # 入口点
│   └── seed/             # 测试数据生成器
├── scripts/
│   ├── demo.sh           # 一键运行所有测试脚本
│   ├── test-competitions.sh  # 比赛CRUD + 开始/结束
│   ├── test-challenges.sh    # 题目CRUD
│   ├── test-submissions.sh   # Flag提交 + 记录
│   ├── test-leaderboard.sh   # 排行榜
│   ├── test-notifications.sh # 通知
│   ├── test-hints.sh         # 提示CRUD
│   └── test-analytics.sh     # 分析
├── internal/
│   ├── config/           # YAML 配置加载
│   ├── handler/          # HTTP 处理器
│   ├── middleware/        # JWT 认证、管理员关卡
│   ├── model/            # 领域结构体
│   ├── plugin/           # 插件接口
│   ├── service/          # 业务逻辑
│   ├── snowflake/        # ID 生成器
│   ├── store/            # DB 接口 + MySQL 实现
│   └── integration/      # 集成测试
├── plugins/
│   ├── leaderboard/      # 每个比赛的排行榜
│   ├── notification/     # 每个比赛的通知
│   ├── analytics/        # 比赛分析（概览、分类、用户、题目）
│   ├── hints/            # 题目提示系统
│   └── dashboard/        # 比赛仪表盘，含一血
├── sql/schema.sql        # 数据库架构
└── config.yaml           # 配置
```

## 配置

```yaml
server:
  port: 8080

db:
  host: 127.0.0.1
  port: 3306
  user: root
  password: ""
  dbname: ctf

jwt:
  secret: "your-secret-key"
  admin_role: "admin"
```

## 许可证

MIT
