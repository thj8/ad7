# 系统日志 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 CTF 平台添加统一的结构化日志系统，覆盖 Flag 提交、管理员操作、安全事件和系统异常。

**Architecture:** 基于 Go 标准库 `log/slog`，创建 `internal/logger` 包封装初始化和代理函数。stdout 用 TextHandler（开发调试），文件用 JSONHandler（ELK/Loki 收集），MultiHandler 同时写两个目标。配置通过 `config.yaml` 的 `log` 段控制路径和级别。

**Tech Stack:** Go 1.21+ `log/slog`, `os`, `io`

**Spec:** `docs/superpowers/specs/2026-04-19-system-logging-design.md`

---

## File Structure

| 操作 | 文件 | 职责 |
|------|------|------|
| Create | `internal/logger/logger.go` | Init + Info/Warn/Error 代理 |
| Create | `internal/logger/logger_test.go` | 单元测试 |
| Modify | `internal/config/config.go` | 新增 LogConfig 结构体 |
| Modify | `cmd/server/main.go` | 调用 logger.Init 初始化 |
| Modify | `config.yaml.example` | 新增 log 段示例 |
| Modify | `internal/service/submission.go` | Flag 提交日志 |
| Modify | `internal/handler/challenge.go` | 题目管理员操作日志 |
| Modify | `internal/handler/competition.go` | 比赛管理员操作日志 |
| Modify | `plugins/notification/notification.go` | 通知创建日志 |
| Modify | `plugins/hints/hints.go` | 提示创建/更新日志 |
| Modify | `internal/middleware/auth.go` | 认证/权限失败日志 |
| Modify | `internal/middleware/ratelimit.go` | 限流触发日志 |

---

### Task 1: 创建 logger 包

**Files:**
- Create: `internal/logger/logger.go`

- [ ] **Step 1: 创建 logger 包**

```go
// Package logger 提供统一的结构化日志功能。
// 基于 Go 标准库 log/slog，支持同时输出到 stdout（文本）和文件（JSON）。
package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"ad7/internal/config"
)

// Init 根据配置初始化全局 logger。
// 如果 cfg.Path 非空，同时写文件（JSON）和 stdout（Text）。
// 如果 cfg.Path 为空，仅写 stdout（Text）。
func Init(cfg config.LogConfig) error {
	level := parseLevel(cfg.Level)

	opts := &slog.HandlerOptions{Level: level}

	textHandler := slog.NewTextHandler(os.Stdout, opts)

	if cfg.Path == "" {
		slog.SetDefault(slog.New(textHandler))
		return nil
	}

	// 创建日志文件目录
	dir := filepath.Dir(cfg.Path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	jsonHandler := slog.NewJSONHandler(f, opts)
	handler := newMultiHandler(textHandler, jsonHandler)
	slog.SetDefault(slog.New(handler))
	return nil
}

// multiHandler 同时写入多个 slog.Handler。
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(r slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(r); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return newMultiHandler(handlers...)
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return newMultiHandler(handlers...)
}

// parseLevel 将字符串转为 slog.Level，默认 info。
func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Info 记录 Info 级别日志。
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn 记录 Warn 级别日志。
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error 记录 Error 级别日志。
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/logger/`
Expected: 编译失败，因为 config 包还没有 LogConfig 字段。先修复 config。

- [ ] **Step 3: 添加 LogConfig 到 config 包**

在 `internal/config/config.go` 的 `Config` 结构体中添加 `Log` 字段，并添加 `LogConfig` 结构体定义。

在 `Config` 结构体中添加：
```go
Log LogConfig `yaml:"log"`
```

在 `RateLimitConfig` 之后添加：
```go
// LogConfig 定义日志输出配置。
type LogConfig struct {
	Path  string `yaml:"path"`  // 日志文件路径，空则仅输出到 stdout
	Level string `yaml:"level"` // 日志级别：debug / info / warn / error
}
```

在 `Load` 函数中，`rate limit` 默认值之后添加日志级别默认值：
```go
// 设置默认日志级别
if cfg.Log.Level == "" {
	cfg.Log.Level = "info"
}
```

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 5: 提交**

```bash
git add internal/logger/logger.go internal/config/config.go
git commit -m "feat: add logger package with slog multi-handler"
```

---

### Task 2: 初始化 logger 并更新配置示例

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `config.yaml.example`

- [ ] **Step 1: 在 main.go 中初始化 logger**

在 `cmd/server/main.go` 中添加 import `"ad7/internal/logger"`，在 `config.Load` 之后、数据库连接之前添加：

```go
// 初始化日志系统（stdout + 可选文件输出）
if err := logger.Init(cfg.Log); err != nil {
	log.Fatalf("init logger: %v", err)
}
```

同时将启动日志改为使用 logger：
```go
logger.Info("server starting", "port", cfg.Server.Port)
```

- [ ] **Step 2: 更新 config.yaml.example**

在文件末尾添加：
```yaml

log:
  # path: "logs/server.log"    # 日志文件路径，注释掉则仅输出到 stdout
  level: "info"                # debug / info / warn / error
```

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: 提交**

```bash
git add cmd/server/main.go config.yaml.example
git commit -m "feat: initialize logger in server startup"
```

---

### Task 3: Flag 提交日志

**Files:**
- Modify: `internal/service/submission.go`

- [ ] **Step 1: 添加日志到 SubmitInComp**

在 `internal/service/submission.go` 中添加 import `"ad7/internal/logger"`。

在 `SubmitInComp` 函数中，在 `return ResultCorrect, nil` 之前添加：
```go
logger.Info("flag submitted", "user", req.UserID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "correct")
```

在 `return ResultIncorrect, nil` 之前添加：
```go
logger.Info("flag submitted", "user", req.UserID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "incorrect")
```

在 `return ResultAlreadySolved, nil` 之前添加：
```go
logger.Info("flag submitted", "user", req.UserID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "already_solved")
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 3: 提交**

```bash
git add internal/service/submission.go
git commit -m "feat: add flag submission logging"
```

---

### Task 4: 题目管理员操作日志

**Files:**
- Modify: `internal/handler/challenge.go`

- [ ] **Step 1: 添加日志到 Create/Update/Delete**

在 `internal/handler/challenge.go` 中添加 import `"ad7/internal/logger"` 和 `"ad7/internal/middleware"`。

在 `Create` 函数中，`writeJSON(w, http.StatusCreated, ...)` 之前添加：
```go
logger.Info("challenge created", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "challenge_id", id, "title", req.Title)
```

在 `Update` 函数中，`w.WriteHeader(http.StatusNoContent)` 之前添加：
```go
logger.Info("challenge updated", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "challenge_id", id)
```

在 `Delete` 函数中，`w.WriteHeader(http.StatusNoContent)` 之前添加：
```go
logger.Info("challenge deleted", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "challenge_id", id)
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 3: 提交**

```bash
git add internal/handler/challenge.go
git commit -m "feat: add challenge admin operation logging"
```

---

### Task 5: 比赛管理员操作日志

**Files:**
- Modify: `internal/handler/competition.go`

- [ ] **Step 1: 添加日志到 Create/Update/Delete/AddChallenge/RemoveChallenge**

在 `internal/handler/competition.go` 中添加 import `"ad7/internal/logger"` 和 `"ad7/internal/middleware"`。

在 `Create` 函数中，`writeJSON(w, http.StatusCreated, ...)` 之前添加：
```go
logger.Info("competition created", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id, "title", req.Title)
```

在 `Update` 函数中，`w.WriteHeader(http.StatusNoContent)` 之前添加：
```go
logger.Info("competition updated", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id)
```

在 `Delete` 函数中，`w.WriteHeader(http.StatusNoContent)` 之前添加：
```go
logger.Info("competition deleted", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id)
```

在 `AddChallenge` 函数中，`writeJSON(w, http.StatusCreated, ...)` 之前添加：
```go
logger.Info("challenge assigned", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", compID, "challenge_id", body.ChallengeID)
```

在 `RemoveChallenge` 函数中，`w.WriteHeader(http.StatusNoContent)` 之前添加：
```go
logger.Info("challenge removed", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", compID, "challenge_id", chalID)
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 3: 提交**

```bash
git add internal/handler/competition.go
git commit -m "feat: add competition admin operation logging"
```

---

### Task 6: 插件管理员操作日志

**Files:**
- Modify: `plugins/notification/notification.go`
- Modify: `plugins/hints/hints.go`

- [ ] **Step 1: 添加日志到 notification.createForComp**

在 `plugins/notification/notification.go` 中添加 import `"ad7/internal/logger"` 和 `"ad7/internal/middleware"`。

在 `createForComp` 函数中，`w.WriteHeader(http.StatusCreated)` 之前添加：
```go
logger.Info("notification created", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", compID)
```

- [ ] **Step 2: 添加日志到 hints.create 和 hints.update**

在 `plugins/hints/hints.go` 中添加 import `"ad7/internal/logger"` 和 `"ad7/internal/middleware"`。

在 `create` 函数中，`w.WriteHeader(http.StatusCreated)` 之前添加：
```go
logger.Info("hint created", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "challenge_id", chalID)
```

在 `update` 函数中，`w.WriteHeader(http.StatusNoContent)` 之前添加：
```go
logger.Info("hint updated", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "hint_id", hintID)
```

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: 提交**

```bash
git add plugins/notification/notification.go plugins/hints/hints.go
git commit -m "feat: add plugin admin operation logging"
```

---

### Task 7: 安全事件日志

**Files:**
- Modify: `internal/middleware/auth.go`
- Modify: `internal/middleware/ratelimit.go`

- [ ] **Step 1: 添加日志到 auth.go**

在 `internal/middleware/auth.go` 中添加 import `"ad7/internal/logger"`。

在 `Authenticate` 中，`http.Error(w, '{"error":"invalid token"}', ...)` 之前添加：
```go
logger.Warn("auth failed", "error", "invalid token")
```

同样在 `http.Error(w, '{"error":"missing token"}', ...)` 之前添加：
```go
logger.Warn("auth failed", "error", "missing token")
```

在 `http.Error(w, '{"error":"invalid claims"}', ...)` 之前添加：
```go
logger.Warn("auth failed", "error", "invalid claims")
```

在 `RequireAdmin` 中，`http.Error(w, '{"error":"forbidden"}', ...)` 之前添加：
```go
logger.Warn("access denied", "user", userID, "role", role, "required_role", a.adminRole)
```

注意：`RequireAdmin` 函数需要从 context 获取 userID，当前函数体中只获取了 role。需要在 `role` 赋值后添加：
```go
userID, _ := r.Context().Value(CtxUserID).(string)
```

- [ ] **Step 2: 添加限流日志到 ratelimit.go**

在 `internal/middleware/ratelimit.go` 中，httprate 支持 `WithLimitHandler` 回调。修改 `LimitByUserID` 函数，添加 `httprate.WithLimitHandler` 选项：

在 import 中添加 `"ad7/internal/logger"`。

在 `LimitByUserID` 函数中，在 `httprate.Limit(` 调用中添加选项：
```go
httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
	logger.Warn("rate limited", "user", UserID(r), "ip", r.RemoteAddr, "endpoint", r.URL.Path)
	http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
}),
```

注意：需要从 `httprate.Limit` 的返回逻辑中移除默认的 429 响应，改由 limit handler 处理。

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: 提交**

```bash
git add internal/middleware/auth.go internal/middleware/ratelimit.go
git commit -m "feat: add security event logging"
```

---

### Task 8: Logger 单元测试

**Files:**
- Create: `internal/logger/logger_test.go`

- [ ] **Step 1: 编写测试**

```go
package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ad7/internal/config"
)

func TestInitStdoutOnly(t *testing.T) {
	err := Init(config.LogConfig{Path: "", Level: "info"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	// 应该不 panic
	Info("test message", "key", "value")
}

func TestInitWithFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	err := Init(config.LogConfig{Path: path, Level: "info"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("file test message", "key", "value")

	// 验证文件已创建
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("log file not created")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"", "INFO"},
		{"unknown", "INFO"},
	}
	for _, tt := range tests {
		got := parseLevel(tt.input)
		if got.String() != tt.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLogFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	err := Init(config.LogConfig{Path: path, Level: "debug"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("hello", "key", "value")

	// 读取文件内容验证 JSON 格式
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"msg":"hello"`) {
		t.Errorf("log file should contain msg=hello, got: %s", content)
	}
	if !strings.Contains(content, `"key":"value"`) {
		t.Errorf("log file should contain key=value, got: %s", content)
	}
}

func TestLevelFiltering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	err := Init(config.LogConfig{Path: path, Level: "warn"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("should be filtered")
	Warn("should appear", "key", "value")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "should be filtered") {
		t.Error("Info should be filtered at warn level")
	}
	if !strings.Contains(content, "should appear") {
		t.Error("Warn should appear at warn level")
	}
}
```

- [ ] **Step 2: 运行测试**

Run: `go test ./internal/logger/ -v -count=1`
Expected: ALL PASS

- [ ] **Step 3: 提交**

```bash
git add internal/logger/logger_test.go
git commit -m "test: add logger package unit tests"
```

---

### Task 9: 集成测试验证 + 最终提交

**Files:**
- Modify: `internal/integration/integration_test.go`（如需要）

- [ ] **Step 1: 运行编译检查**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 2: 运行 go vet**

Run: `go vet ./...`
Expected: NO ISSUES

- [ ] **Step 3: 运行所有单元测试**

Run: `go test ./internal/logger/ ./internal/config/ -v -count=1`
Expected: ALL PASS

- [ ] **Step 4: 最终提交（如有遗漏修复）**

```bash
git add -A
git commit -m "test: verify logging integration"
```
