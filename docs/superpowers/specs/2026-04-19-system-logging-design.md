# 系统日志设计

## 概述

为 CTF 比赛平台添加统一的结构化日志系统，覆盖 Flag 提交、管理员操作、安全事件和系统异常四类关键操作。

## 技术选型

基于 Go 1.21+ 标准库 `log/slog`，零第三方依赖。

- stdout 输出文本格式（方便开发调试）
- 文件输出 JSON 格式（方便 ELK/Loki 收集）
- `slog.MultiHandler` 同时写两个目标

## 配置

`config.yaml` 新增 `log` 段：

```yaml
log:
  path: "logs/server.log"    # 日志文件路径，空则只输出到 stdout
  level: "info"              # debug / info / warn / error
```

Go 结构体：

```go
type LogConfig struct {
    Path  string `yaml:"path"`   // 日志文件路径
    Level string `yaml:"level"`  // 日志级别
}
```

默认值：`path=""`（仅 stdout），`level="info"`。

## 模块结构

```
internal/logger/
├── logger.go    # Init() + Info/Warn/Error 代理函数
```

### Init(cfg LogConfig) error

1. 根据 level 字符串创建 `slog.LevelVar`
2. 如果 path 非空：`os.MkdirAll` 创建目录，打开文件
3. 创建两个 handler：stdout 用 `slog.NewTextHandler`，文件用 `slog.NewJSONHandler`
4. `slog.SetDefault(slog.New(slog.MultiHandler(textHandler, fileHandler)))`

### 代理函数

```go
func Info(msg string, args ...any)  // slog.Info 代理
func Warn(msg string, args ...any)  // slog.Warn 代理
func Error(msg string, args ...any) // slog.Error 代理
```

调用示例：
```go
logger.Info("flag submitted", "user", uid, "challenge", chalID, "competition", compID, "result", "correct")
```

输出示例：
- stdout: `2026-04-19T10:30:00+08:00 INFO flag submitted user=player_001 challenge=abc123 result=correct`
- file:   `{"time":"2026-04-19T10:30:00+08:00","level":"INFO","msg":"flag submitted","user":"player_001","challenge":"abc123","result":"correct"}`

## 初始化时机

在 `cmd/server/main.go` 中，`config.Load` 之后、数据库连接之前调用 `logger.Init`。

## 日志埋点位置

### Flag 提交

| 文件 | 函数 | 级别 | 消息 | 字段 |
|------|------|------|------|------|
| service/submission.go | SubmitInComp | Info | `flag submitted` | user, challenge, competition, result |

result 取值：`correct` / `incorrect` / `already_solved`。

记录在 service 层而非 handler 层，因为 service 层有完整的业务上下文（提交结果）。

### 管理员操作

| 文件 | 函数 | 级别 | 消息 | 字段 |
|------|------|------|------|------|
| handler/challenge.go | Create | Info | `challenge created` | user, role, challenge_id, title |
| handler/challenge.go | Update | Info | `challenge updated` | user, role, challenge_id |
| handler/challenge.go | Delete | Info | `challenge deleted` | user, role, challenge_id |
| handler/competition.go | Create | Info | `competition created` | user, role, competition_id, title |
| handler/competition.go | Update | Info | `competition updated` | user, role, competition_id |
| handler/competition.go | Delete | Info | `competition deleted` | user, role, competition_id |
| handler/competition.go | AddChallenge | Info | `challenge assigned` | user, competition_id, challenge_id |
| handler/competition.go | RemoveChallenge | Info | `challenge removed` | user, competition_id, challenge_id |
| handler/notification.go | Create | Info | `notification created` | user, competition_id |
| handler/hints.go | Create | Info | `hint created` | user, challenge_id |
| handler/hints.go | Update | Info | `hint updated` | user, challenge_id |

### 安全事件

| 文件 | 函数 | 级别 | 消息 | 字段 |
|------|------|------|------|------|
| middleware/auth.go | Authenticate（token 无效） | Warn | `auth failed` | error |
| middleware/auth.go | RequireAdmin（权限不足） | Warn | `access denied` | user, role, required_role |
| middleware/ratelimit.go | 限流触发 | Warn | `rate limited` | user, ip, endpoint |

### 系统异常

| 文件 | 位置 | 级别 | 消息 | 字段 |
|------|------|------|------|------|
| service 层各函数 | store 调用返回 error 时 | Error | `{operation} failed` | operation, error |
| cmd/server/main.go | 启动失败 | Error（log.Fatalf 保留） | 已有 log.Fatalf | — |

service 层在调用 store 返回 error 时，用 `logger.Error` 记录操作名和错误信息。`sql.ErrNoRows` 属于正常业务逻辑（如"题目不存在"），不记录。

## 测试

- `internal/logger/logger_test.go`：测试 Init 配置解析、MultiHandler 创建、级别过滤
- 集成测试中验证日志输出不影响现有功能

## 不做的事

- 不做日志轮转（由运维用 logrotate 或 docker logging driver 处理）
- 不做远程日志发送
- 不引入第三方日志库
- 不修改现有的 `chimw.Logger` 请求日志中间件
