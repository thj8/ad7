# CTF 解题赛系统 - 后端设计

## 概述

Jeopardy 模式 CTF 解题赛系统的后端 API 服务。只实现核心功能：题目管理和 flag 提交验证。用户系统由中台通过 JWT Token 提供，排行榜等扩展功能后期以插件形式接入。

## 技术栈

- **语言**：Go
- **路由**：`go-chi/chi/v5`
- **数据库**：MySQL (`database/sql` + `go-sql-driver/mysql`)
- **认证**：`golang-jwt/jwt/v5`（解析中台下发的 JWT Token）
- **配置**：YAML 文件

## 项目结构

```
ad7/
├── cmd/
│   └── server/
│       └── main.go            # 入口，启动 HTTP 服务
├── internal/
│   ├── config/
│   │   └── config.go          # 配置加载
│   ├── middleware/
│   │   └── auth.go            # JWT 认证中间件
│   ├── model/
│   │   └── challenge.go       # 数据结构定义
│   ├── handler/
│   │   ├── challenge.go       # 题目 CRUD API
│   │   └── submission.go      # 提交 flag API
│   ├── service/
│   │   ├── challenge.go       # 题目业务逻辑
│   │   └── submission.go      # 提交业务逻辑
│   └── store/
│       └── mysql.go           # 数据库初始化 + 查询
├── sql/
│   └── schema.sql             # 建表 SQL
├── config.yaml                # 配置文件示例
├── go.mod
└── go.sum
```

**分层职责**：

- `handler`：HTTP 参数解析和响应序列化，不含业务逻辑
- `service`：业务逻辑（验证 flag、判断重复提交等）
- `store`：SQL 查询，数据库交互
- `model`：struct 定义，无外部依赖

## 数据库设计

### challenges 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INT AUTO_INCREMENT | 主键 |
| title | VARCHAR(255) | 题目标题 |
| category | VARCHAR(64) | 分类，默认 misc |
| description | TEXT | 题目描述 |
| score | INT | 分值，默认 100 |
| flag | VARCHAR(255) | 正确 flag |
| is_enabled | TINYINT(1) | 是否上架，默认 1 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### submissions 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INT AUTO_INCREMENT | 主键 |
| user_id | VARCHAR(128) | JWT 解析出的用户标识 |
| challenge_id | INT | 关联题目 |
| submitted_flag | VARCHAR(255) | 用户提交的 flag |
| is_correct | TINYINT(1) | 是否正确 |
| created_at | DATETIME | 提交时间 |

索引：`idx_user_challenge (user_id, challenge_id)` 用于查重和统计。

建表 SQL：

```sql
CREATE TABLE challenges (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    category    VARCHAR(64)  NOT NULL DEFAULT 'misc',
    description TEXT         NOT NULL,
    score       INT          NOT NULL DEFAULT 100,
    flag        VARCHAR(255) NOT NULL,
    is_enabled  TINYINT(1)   NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE submissions (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    user_id        VARCHAR(128) NOT NULL,
    challenge_id   INT          NOT NULL,
    submitted_flag VARCHAR(255) NOT NULL,
    is_correct     TINYINT(1)   NOT NULL,
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (challenge_id) REFERENCES challenges(id),
    INDEX idx_user_challenge (user_id, challenge_id)
);
```

## API 设计

所有接口前缀 `/api/v1`，JSON 请求/响应。

### 用户端接口（需 JWT 认证）

**GET /api/v1/challenges**

获取已启用的题目列表，不含 flag 字段。

响应：
```json
{
  "challenges": [
    {
      "id": 1,
      "title": "Hello World",
      "category": "misc",
      "description": "flag is flag{hello}",
      "score": 100
    }
  ]
}
```

**GET /api/v1/challenges/{id}**

获取单个题目详情，不含 flag 字段。404 如果不存在或未启用。

响应：
```json
{
  "id": 1,
  "title": "Hello World",
  "category": "misc",
  "description": "flag is flag{hello}",
  "score": 100
}
```

**POST /api/v1/challenges/{id}/submit**

提交 flag。已解过的题目返回 `already_solved`。

请求：
```json
{"flag": "flag{hello_world}"}
```

响应：
```json
{"success": true, "message": "correct"}
```
```json
{"success": false, "message": "incorrect"}
```
```json
{"success": false, "message": "already_solved"}
```

### 管理端接口（需 JWT 且 role == admin）

**POST /api/v1/admin/challenges**

创建题目。

请求：
```json
{
  "title": "Hello World",
  "category": "misc",
  "description": "flag is flag{hello}",
  "score": 100,
  "flag": "flag{hello}"
}
```

**PUT /api/v1/admin/challenges/{id}**

更新题目，请求体同创建（字段均可选，只更新传入的字段）。

**DELETE /api/v1/admin/challenges/{id}**

删除题目。

**GET /api/v1/admin/submissions**

查看提交记录，支持 `?user_id=&challenge_id=` 筛选。

响应：
```json
{
  "submissions": [
    {
      "id": 1,
      "user_id": "user123",
      "challenge_id": 1,
      "submitted_flag": "flag{wrong}",
      "is_correct": false,
      "created_at": "2026-04-17T10:00:00Z"
    }
  ]
}
```

## 认证流程

```
请求 → JWT 中间件
         ├─ 从 Authorization: Bearer <token> 取 token
         ├─ 验证签名（HS256，secret 从配置读）
         ├─ 检查过期
         ├─ 提取 claims["sub"] → user_id
         ├─ 提取 claims["role"] → role
         └─ 写入 request context
                │
                ▼
         路由匹配
         ├─ /api/v1/admin/* → 检查 role == config.jwt.admin_role，否则 403
         └─ /api/v1/* → 直接通过
```

- `user_id` 从 JWT 标准声明 `sub` 字段取
- 管理员通过 `role` 字段与配置的 `admin_role` 比对判断
- 不关心用户注册/密码，完全信任中台 token

## 配置

`config.yaml`：

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

配置文件路径通过命令行 `-config` 参数指定，默认 `config.yaml`。

## 不在范围内

- 排行榜（后期插件）
- 用户注册/登录（中台提供）
- 动态 flag / 容器下发
- 题目附件/文件上传
- 前端页面（独立项目）
