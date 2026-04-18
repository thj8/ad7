---
name: Rate Limiting Design
description: Design document for implementing rate limiting on flag submission endpoints
type: design
---

# Rate Limiting 设计文档

## 概述

为提交 Flag 接口添加频率限制功能，防止用户暴力猜解 Flag。使用 `go-chi/httprate` 库实现，支持按用户 ID 和按 IP 两种限制方式。

## 需求

- 提交 Flag 接口限制：10 秒内最多 3 次
- 支持按用户 ID 限制（主要）和按 IP 限制（备用）
- 参数可通过配置文件配置，同时有默认值
- 使用内存存储（单实例部署）

## 技术选型

- **库**：`github.com/go-chi/httprate` - 滑动窗口计数器， inspired by Cloudflare
- **存储**：内存存储（默认）
- **位置**：HTTP 中间件层

## 架构设计

### 1. 配置结构 (`internal/config/config.go`)

```go
type Config struct {
    // ... 现有字段 ...
    RateLimit RateLimitConfig `yaml:"rate_limit"`
}

type RateLimitConfig struct {
    Submission RateLimitRule `yaml:"submission"`
}

type RateLimitRule struct {
    Requests int           `yaml:"requests"` // 请求数
    Window   time.Duration `yaml:"window"`   // 时间窗口
}
```

默认值：
- `Requests: 3`
- `Window: 10 * time.Second`

### 2. 中间件实现 (`internal/middleware/ratelimit.go`)

```go
package middleware

import (
    "net/http"
    "time"

    "github.com/go-chi/httprate"
)

// LimitByIP 创建按 IP 限制的中间件
func LimitByIP(requests int, window time.Duration) func(http.Handler) http.Handler {
    return httprate.Limit(
        requests,
        window,
        httprate.WithKeyFuncs(httprate.KeyByIP),
    )
}

// LimitByUserID 创建按用户 ID 限制的中间件（需要先经过 Authenticate 中间件）
func LimitByUserID(requests int, window time.Duration) func(http.Handler) http.Handler {
    return httprate.Limit(
        requests,
        window,
        httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
            userID := UserID(r)
            if userID == "" {
                // fallback to IP if no user ID
                return httprate.KeyByIP(r)
            }
            return userID, nil
        }),
    )
}
```

### 3. 路由注册 (`cmd/server/main.go`)

在提交 Flag 路由上应用中间件：

```go
r.With(
    middleware.LimitByUserID(
        cfg.RateLimit.Submission.Requests,
        cfg.RateLimit.Submission.Window,
    ),
).Post("/competitions/{comp_id}/challenges/{id}/submit", submissionH.SubmitInComp)
```

### 4. 配置文件 (`config.yaml`)

```yaml
rate_limit:
  submission:
    requests: 3
    window: 10s
```

## 测试计划

添加集成测试：

1. **正常提交**：验证 10 秒内 3 次提交都成功
2. **超过限制**：验证第 4 次提交返回 429 Too Many Requests
3. **窗口重置**：验证等待超过 10 秒后可以继续提交

## 变更文件列表

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `go.mod` | 修改 | 添加 `github.com/go-chi/httprate` 依赖 |
| `internal/config/config.go` | 修改 | 添加 `RateLimit` 配置结构 |
| `internal/middleware/ratelimit.go` | 新增 | 频率限制中间件 |
| `cmd/server/main.go` | 修改 | 应用频率限制中间件到提交 Flag 路由 |
| `config.yaml` | 修改 | 添加频率限制配置 |
| `config.yaml.example` | 修改 | 添加频率限制配置示例 |
| `internal/integration/integration_test.go` | 修改 | 添加频率限制集成测试 |
