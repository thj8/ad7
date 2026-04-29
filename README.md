# AD7 - CTF Jeopardy 平台

一个使用 Go 构建的多比赛 CTF（夺旗赛）Jeopardy 平台。

## 功能特性

- **多比赛支持** — 创建和管理多个独立的比赛，每个比赛有自己的题目集合
- **个人/队伍双模式** — 支持个人赛和队伍赛两种模式
  - 个人模式：以用户为单位统计分数
  - 队伍模式：以队伍为单位统计分数，单个队员的提交计入全队
- **灵活的队伍加入方式** — 队伍模式下支持两种加入方式
  - 自由加入：任何有队伍的用户都可以进入比赛
  - 管理模式：仅管理员添加的队伍可进入比赛
- **题目管理** — 管理员 CRUD 操作，使用静态 Flag 验证
- **比赛内 Flag 提交** — 用户在比赛上下文中提交 Flag，防止重复解题（队伍模式下按队伍去重）
- **每个比赛的排行榜** — 按总分降序排列，同分按最早解题时间打破平局，自动切换个人/队伍模式
- **一血/二血/三血追踪** — 记录每道题的前三名解题者（支持个人和队伍两种模式）
- **每个比赛的通知** — 管理员可以发布限定在比赛范围内的公告
- **比赛分析** — 详细统计数据，包括概览、分类、用户/队伍和题目分析
- **题目提示系统** — 管理员管理提示，用户按需查看可见提示
- **插件系统** — 可扩展的编译时插件接口，用于添加新功能
- **UUID ID** — 所有公开 ID 使用 UUID v4（32字符十六进制字符串无连字符）作为唯一标识符
- **独立认证服务** — 认证服务器独立部署，支持用户注册/登录、队伍管理、队长权限

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

# 运行迁移（如果从旧版本升级）
mysql -u root -p your_db < sql/migrations/001_team_members.sql
mysql -u root -p your_db < sql/migrations/002_team_competition_mode.sql

# 生成测试数据（可选）
go run ./cmd/seed/

# 运行测试
go test ./...

# 仅运行集成测试（需要 MySQL）
source .env
go test ./internal/integration/... -v -count=1

# 运行服务器（必须按顺序启动）
# 1. 先启动认证服务器（端口 8081）
go run ./cmd/auth-server -config cmd/auth-server/config.yaml

# 2. 再启动 CTF 服务器（端口 8080）
go run ./cmd/server -config config.yaml
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
| POST | `/api/v1/challenges` | 创建题目 |
| PUT | `/api/v1/challenges/{id}` | 更新题目 |
| DELETE | `/api/v1/challenges/{id}` | 删除题目 |

### 题目（用户）

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/api/v1/challenges` | 列出已启用的题目 |
| GET | `/api/v1/challenges/{id}` | 获取题目详情 |

### 提交

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| POST | `/api/v1/submissions` | 提交 Flag（全局） |
| POST | `/api/v1/competitions/{id}/submit` | 提交 Flag（比赛内，速率限制） |

### 比赛（管理员）

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| POST | `/api/v1/competitions` | 创建比赛（支持 mode 和 team_join_mode） |
| PUT | `/api/v1/competitions/{id}` | 更新比赛 |
| DELETE | `/api/v1/competitions/{id}` | 删除比赛 |
| GET | `/api/v1/competitions` | 列出所有比赛 |
| POST | `/api/v1/competitions/{id}/start` | 开始比赛 |
| POST | `/api/v1/competitions/{id}/end` | 结束比赛 |
| GET | `/api/v1/competitions/{id}/teams` | 列出比赛队伍（管理员，队伍模式） |
| POST | `/api/v1/competitions/{id}/teams` | 添加队伍到比赛（管理员，队伍模式） |
| DELETE | `/api/v1/competitions/{id}/teams/:teamID` | 从比赛移除队伍（管理员，队伍模式） |

### 比赛（用户）

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/api/v1/competitions` | 列出活跃比赛 |
| GET | `/api/v1/competitions/{id}` | 获取比赛详情 |
| GET | `/api/v1/competitions/{id}/challenges` | 列出比赛题目（含访问控制） |

### 插件

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/api/v1/competitions/{id}/leaderboard` | 比赛排行榜（自动切换个人/队伍模式） |
| GET | `/api/v1/topthree/competitions/{id}` | 比赛每道题的前三名（支持个人/队伍模式） |
| POST | `/api/v1/admin/competitions/{id}/notifications` | 创建比赛通知 |
| GET | `/api/v1/competitions/{id}/notifications` | 列出比赛通知 |
| GET | `/api/v1/competitions/{id}/analytics` | 比赛分析（支持个人/队伍模式） |
| POST | `/api/v1/admin/challenges/{id}/hints` | 创建题目提示 |
| PUT | `/api/v1/admin/hints/{id}` | 更新题目提示 |
| DELETE | `/api/v1/admin/hints/{id}` | 删除题目提示 |
| GET | `/api/v1/challenges/{id}/hints` | 列出可见的题目提示 |

### 比赛分析

分析插件为比赛提供详细统计数据，自动适应个人/队伍模式：

**概览（`/analytics`）**
- 总用户/队伍数、题目数、提交数
- 正确提交数量
- 平均每队/每人解题数
- 平均解题时间（从比赛开始计算）
- 完成率（平均每队/每人解题百分比）

**按分类（`/analytics` → categories）**
- 每个分类的总题目数
- 每个分类的总解题数
- 每个分类的独立解题用户/队伍数
- 平均每队/每人解题数
- 成功率（正确 / 总提交数）

**用户/队伍统计（`/analytics` → users/teams）**
- 每个用户/队伍的总解题数、总分、总尝试数
- 成功率
- 第一次和最后一次解题时间
- 按总分降序、第一次解题时间升序排列

**题目统计（`/analytics` → challenges）**
- 每个题目的总解题数、尝试数、成功率
- 独立解题用户/队伍数
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
│   ├── server/           # CTF 服务器入口点（端口 8080）
│   ├── auth-server/      # 认证服务器入口点（端口 8081）
│   └── seed/             # 测试数据生成器
├── internal/
│   ├── auth/             # 认证模块（用户、队伍、JWT）
│   ├── config/           # YAML 配置加载
│   ├── handler/          # HTTP 处理器
│   ├── middleware/       # 认证、速率限制
│   ├── model/            # 领域结构体
│   ├── plugin/           # 插件接口
│   ├── pluginutil/       # 插件共享工具和查询
│   ├── event/            # 事件系统（pub/sub）
│   ├── service/          # 业务逻辑（含 team_resolver）
│   ├── uuid/             # UUID 生成器
│   ├── logger/           # 结构化日志
│   ├── store/            # DB 接口 + MySQL 实现
│   └── integration/      # 集成测试
├── plugins/
│   ├── leaderboard/      # 每个比赛的排行榜（双模式）
│   ├── topthree/         # 一血/二血/三血追踪（双模式）
│   ├── notification/     # 每个比赛的通知
│   ├── analytics/        # 比赛分析（双模式）
│   └── hints/            # 题目提示系统
├── sql/
│   ├── schema.sql        # 数据库架构
│   └── migrations/       # 迁移脚本
├── docs/                 # 文档
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
