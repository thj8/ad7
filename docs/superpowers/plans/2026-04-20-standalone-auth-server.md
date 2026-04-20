# Standalone Auth Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the auth module into a standalone HTTP server so the CTF project communicates with auth exclusively via HTTP.

**Architecture:** Two HTTP servers in the same Go module — `cmd/auth-server/` (port 8081) owns all user/team/auth logic; `cmd/server/` (port 8080) calls auth's `/verify` endpoint for JWT validation instead of parsing tokens locally. Both share the same MySQL database.

**Tech Stack:** Go, chi router, JWT (golang-jwt/v5), bcrypt, MySQL

---

## File Map

### New files
| File | Responsibility |
|------|---------------|
| `cmd/auth-server/main.go` | Auth server entrypoint |
| `cmd/auth-server/config.yaml` | Auth server config |
| `internal/auth/verify_handler.go` | `POST /api/v1/verify` handler + service method |
| `internal/auth/verify_handler_test.go` | Unit tests for verify handler |

### Modified files
| File | Change |
|------|--------|
| `internal/middleware/auth.go` | `Authenticate` calls auth `/verify` via HTTP instead of local JWT parsing |
| `internal/middleware/auth_test.go` | Update tests for new HTTP-based auth |
| `internal/middleware/ratelimit_test.go` | Fix `NewAuth` call signature |
| `internal/config/config.go` | Add `AuthURL` field |
| `cmd/server/main.go` | Remove auth imports, pass auth URL to middleware |
| `internal/testutil/testutil.go` | Spin up auth test server, update `NewAuth` calls |
| `CLAUDE.md` | Document new server and auth.url config |

---

### Task 1: Add verify endpoint to auth service

**Files:**
- Modify: `internal/auth/service.go`
- Create: `internal/auth/verify_handler.go`
- Create: `internal/auth/verify_handler_test.go`

The verify endpoint validates a JWT token and returns `{user_id, role}`.

- [ ] **Step 1: Write failing test for Verify handler**

Create `internal/auth/verify_handler_test.go`:

```go
package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerify_ValidToken(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewVerifyHandler(svc)

	// Generate a valid token
	token, err := svc.GenerateToken("user123", "member")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/verify", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.Verify(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["user_id"] != "user123" {
		t.Errorf("user_id = %q, want %q", resp["user_id"], "user123")
	}
	if resp["role"] != "member" {
		t.Errorf("role = %q, want %q", resp["role"], "member")
	}
}

func TestVerify_MissingToken(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewVerifyHandler(svc)

	req := httptest.NewRequest("POST", "/api/v1/verify", nil)
	w := httptest.NewRecorder()

	handler.Verify(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestVerify_InvalidToken(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewVerifyHandler(svc)

	req := httptest.NewRequest("POST", "/api/v1/verify", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	handler.Verify(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	store := newMockUserStore()
	svc := &AuthService{
		users:    store,
		secret:   []byte("test-secret"),
		adminRole: "admin",
		tokenTTL: -1 * time.Hour, // already expired
	}
	handler := NewVerifyHandler(svc)

	token, _ := svc.GenerateToken("user123", "member")

	req := httptest.NewRequest("POST", "/api/v1/verify", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.Verify(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestVerify -v`
Expected: compilation error — `NewVerifyHandler` not defined

- [ ] **Step 3: Implement VerifyHandler and Verify method**

Create `internal/auth/verify_handler.go`:

```go
package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// VerifyHandler 处理 token 验证请求。
type VerifyHandler struct {
	svc *AuthService
}

// NewVerifyHandler 创建 VerifyHandler 实例。
func NewVerifyHandler(svc *AuthService) *VerifyHandler {
	return &VerifyHandler{svc: svc}
}

// Verify 处理 POST /api/v1/verify 请求。
// 从 Authorization 头提取 Bearer token，验证后返回 {user_id, role}。
func (h *VerifyHandler) Verify(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		authWriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
		return
	}
	tokenStr := strings.TrimPrefix(header, "Bearer ")

	userID, role, err := h.svc.VerifyToken(tokenStr)
	if err != nil {
		authWriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	authWriteJSON(w, http.StatusOK, map[string]string{
		"user_id": userID,
		"role":    role,
	})
}
```

Add `VerifyToken` method to `internal/auth/service.go` (append after `GenerateToken`):

```go
// VerifyToken 验证 JWT token，返回 user_id 和 role。
func (s *AuthService) VerifyToken(tokenStr string) (string, string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return "", "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", jwt.ErrSignatureInvalid
	}
	userID, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	if userID == "" || role == "" {
		return "", "", jwt.ErrSignatureInvalid
	}
	return userID, role, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/... -v -count=1`
Expected: All PASS (including existing 14 + new 4)

- [ ] **Step 5: Commit**

```bash
git add internal/auth/verify_handler.go internal/auth/verify_handler_test.go internal/auth/service.go
git commit -m "feat: 添加 token 验证接口 (POST /api/v1/verify)"
```

---

### Task 2: Create auth server entrypoint

**Files:**
- Create: `cmd/auth-server/main.go`
- Create: `cmd/auth-server/config.yaml`

- [ ] **Step 1: Create auth server config file**

Create `cmd/auth-server/config.yaml`:

```yaml
server:
  port: 8081
db:
  host: "127.0.0.1"
  port: 3306
  user: "root"
  password: ""
  dbname: "ctf"
jwt:
  secret: "change-me-in-production"
  admin_role: "admin"
log:
  level: "info"
```

- [ ] **Step 2: Create auth server main.go**

Create `cmd/auth-server/main.go`:

```go
// Package main 是认证服务的独立 HTTP 服务器入口。
// 提供用户注册/登录、JWT token 验证、队伍管理等功能。
// CTF 主服务通过 HTTP 调用 /api/v1/verify 来验证用户 token。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"ad7/internal/auth"
	"ad7/internal/config"
	"ad7/internal/logger"
	"ad7/internal/middleware"
	"ad7/internal/store"
)

func main() {
	cfgPath := flag.String("config", "cmd/auth-server/config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := logger.Init(cfg.Log); err != nil {
		log.Fatalf("init logger: %v", err)
	}

	st, err := store.New(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer st.Close()

	authMW := middleware.NewAuth(cfg.JWT.Secret, cfg.JWT.AdminRole)

	authStore := auth.NewAuthStore(st.DB())
	authSvc := auth.NewAuthService(authStore, cfg.JWT.Secret, cfg.JWT.AdminRole)
	teamSvc := auth.NewTeamService(authStore, authStore)
	authH := auth.NewAuthHandler(authSvc)
	teamH := auth.NewTeamHandler(teamSvc)
	verifyH := auth.NewVerifyHandler(authSvc)
	authDeps := auth.RouteDeps{Auth: authMW, AuthH: authH, TeamH: teamH}

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	// 公共路由（register, login, verify — 不需要 JWT）
	r.Route("/api/v1", func(r chi.Router) {
		auth.RegisterPublicRoutes(r, authDeps)
		r.Post("/verify", verifyH.Verify)
	})

	// 需要认证的队伍路由
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMW.Authenticate)
		auth.RegisterTeamRoutes(r, authDeps)
	})

	// 管理员队伍路由
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(authMW.Authenticate)
		r.Use(authMW.RequireAdmin)
		auth.RegisterAdminTeamRoutes(r, authDeps)
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	logger.Info("auth server starting", "port", cfg.Server.Port)
	log.Fatal(http.ListenAndServe(addr, r))
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/auth-server/`
Expected: no output (success)

- [ ] **Step 4: Commit**

```bash
git add cmd/auth-server/main.go cmd/auth-server/config.yaml
git commit -m "feat: 创建 auth 独立服务器入口"
```

---

### Task 3: Add auth URL config and refactor CTF middleware

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/middleware/auth.go`

- [ ] **Step 1: Add AuthURL to config**

In `internal/config/config.go`, add `AuthConfig` struct and field to `Config`:

Add new struct after `LogConfig`:

```go
// AuthConfig 定义认证服务的连接参数。
type AuthConfig struct {
	URL string `yaml:"url"` // 认证服务地址，如 "http://localhost:8081"
}
```

Add `Auth AuthConfig` field to the `Config` struct, after `Log`:

```go
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	DB        DBConfig        `yaml:"db"`
	JWT       JWTConfig       `yaml:"jwt"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Log       LogConfig       `yaml:"log"`
	Auth      AuthConfig      `yaml:"auth"`
}
```

Add default in `Load()` function, after the log level default:

```go
if cfg.Auth.URL == "" {
	cfg.Auth.URL = "http://localhost:8081"
}
```

- [ ] **Step 2: Refactor middleware Auth to call auth service /verify**

Replace entire content of `internal/middleware/auth.go`:

```go
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ad7/internal/logger"
)

// contextKey 是上下文键的类型，避免与其它包的键冲突。
type contextKey string

const (
	// CtxUserID 是存储在上下文中的用户 ID 键
	CtxUserID contextKey = "user_id"
	// CtxRole 是存储在上下文中的用户角色键
	CtxRole   contextKey = "role"
)

// Auth 封装认证中间件配置。
type Auth struct {
	authURL   string        // 认证服务地址
	adminRole string        // 管理员角色名称
	client    *http.Client  // HTTP 客户端（带超时）
}

// NewAuth 创建 Auth 中间件实例。
// 参数：
//   - authURL: 认证服务地址（如 "http://localhost:8081"）
//   - adminRole: 管理员角色名称（如 "admin"）
func NewAuth(authURL, adminRole string) *Auth {
	return &Auth{
		authURL:   authURL,
		adminRole: adminRole,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// verifyResponse 是 /verify 接口的响应结构。
type verifyResponse struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// Authenticate 是认证中间件。
// 从请求头提取 Bearer token，调用认证服务的 /api/v1/verify 接口验证，
// 将 user_id 和 role 注入请求上下文。
func (a *Auth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			logger.Warn("auth failed", "error", "missing token")
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}

		userID, role, err := a.verifyToken(r, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			logger.Warn("auth failed", "error", err.Error())
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), CtxUserID, userID)
		ctx = context.WithValue(ctx, CtxRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// verifyToken 调用认证服务验证 token。
func (a *Auth) verifyToken(r *http.Request, tokenStr string) (string, string, error) {
	req, err := http.NewRequestWithContext(r.Context(), "POST", a.authURL+"/api/v1/verify", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("verify returned status %d", resp.StatusCode)
	}

	var vr verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return "", "", err
	}
	return vr.UserID, vr.Role, nil
}

// RequireAdmin 是管理员权限校验中间件。
func (a *Auth) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(CtxRole).(string)
		userID, _ := r.Context().Value(CtxUserID).(string)
		if role != a.adminRole {
			logger.Warn("access denied", "user", userID, "role", role, "required_role", a.adminRole)
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserID 从请求上下文中提取用户 ID。
func UserID(r *http.Request) string {
	v, _ := r.Context().Value(CtxUserID).(string)
	return v
}
```

**Important note:** The `verifyToken` method uses `logger.Error` which returns an error. Check if `logger` package has this. If not, use `fmt.Errorf` instead. Check `internal/logger/` first.

- [ ] **Step 3: Verify build**

Run: `go build ./internal/middleware/ ./cmd/auth-server/`
Expected: no output (success)

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/middleware/auth.go
git commit -m "feat: CTF 中间件改为调用 auth 服务验证 token"
```

---

### Task 4: Update CTF server main and tests

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/testutil/testutil.go`
- Modify: `internal/middleware/ratelimit_test.go`

- [ ] **Step 1: Update cmd/server/main.go — remove auth module, pass auth URL**

Remove `"ad7/internal/auth"` from imports. Remove all auth module initialization code (lines 77-115 in current version). Replace with passing `cfg.Auth.URL` to `NewAuth`:

Change:
```go
authMW := middleware.NewAuth(cfg.JWT.Secret, cfg.JWT.AdminRole)
```
To:
```go
authMW := middleware.NewAuth(cfg.Auth.URL, cfg.JWT.AdminRole)
```

Remove these blocks entirely:
- "初始化认证模块" section (authStore, authSvc, teamSvc, authH, teamH, authDeps)
- "注册认证公共路由" r.Route block
- "注册认证路由组" r.Route block (auth.RegisterTeamRoutes)
- "注册管理员队伍路由" r.Route block (auth.RegisterAdminTeamRoutes)

The final `cmd/server/main.go` should be clean — no references to `internal/auth` package.

- [ ] **Step 2: Update internal/testutil/testutil.go**

Add auth server setup. The test environment needs to start an auth test server alongside the CTF test server.

Add import for `"ad7/internal/auth"`.

In `NewTestEnv`, before the CTF router setup, add auth server:

```go
// 启动 auth 测试服务器
authStore := auth.NewAuthStore(st.DB())
authSvc := auth.NewAuthService(authStore, TestSecret, AdminRole)
teamSvc := auth.NewTeamService(authStore, authStore)
authH := auth.NewAuthHandler(authSvc)
teamH := auth.NewTeamHandler(teamSvc)
verifyH := auth.NewVerifyHandler(authSvc)
authMW := middleware.NewAuth(TestSecret, AdminRole) // auth server uses local JWT for its own routes

authDeps := auth.RouteDeps{Auth: authMW, AuthH: authH, TeamH: teamH}
authR := chi.NewRouter()
authR.Use(chimw.Recoverer)
authR.Route("/api/v1", func(r chi.Router) {
    auth.RegisterPublicRoutes(r, authDeps)
    r.Post("/verify", verifyH.Verify)
})
authServer := httptest.NewServer(authR)
```

Then change the CTF middleware to use auth server URL:
```go
auth := middleware.NewAuth(authServer.URL, AdminRole)
```

Store authServer in `TestEnv` and close it in `Close()`.

Add `users` and `teams` to `Cleanup`:
```go
db.Exec("DELETE FROM users")
db.Exec("DELETE FROM teams")
```

- [ ] **Step 3: Update ratelimit_test.go**

Check `internal/middleware/ratelimit_test.go` for any `NewAuth` calls. Update signature from `NewAuth(secret, role)` to `NewAuth(authURL, role)`. For unit tests, pass a dummy URL like `"http://localhost:8081"` since the rate limiter tests don't test authentication.

- [ ] **Step 4: Verify full build**

Run: `go build ./...`
Expected: no output (success)

- [ ] **Step 5: Run all unit tests**

Run: `go test ./internal/auth/... ./internal/middleware/... -v -count=1`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go internal/testutil/testutil.go internal/middleware/ratelimit_test.go
git commit -m "refactor: CTF 服务移除 auth 直接依赖，改为 HTTP 调用"
```

---

### Task 5: Update CLAUDE.md and final verification

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md**

Update the commands section to add auth server startup:

```
# 运行认证服务器（独立服务）
go run ./cmd/auth-server -config cmd/auth-server/config.yaml
```

Update the architecture section to reflect the separation:

```
- `cmd/auth-server/main.go` — 认证独立服务入口（注册、登录、token 验证、队伍管理）
- `internal/auth/` — 认证模块：用户注册/登录、队伍 CRUD、token 验证。独立 HTTP 服务，CTF 通过 /api/v1/verify 接口调用
```

Update config section:

```
- `internal/config/` — YAML 配置（`server.port`、`db.*`、`jwt.secret`、`jwt.admin_role`、`log.*`、`ratelimit.*`、`auth.url`）
```

Update constraints section:

```
- **auth模块**: 独立 HTTP 服务，CTF 项目只能通过 HTTP 接口调用（/register, /login, /verify, /teams/*）
```

- [ ] **Step 2: Full build and test**

Run: `go build ./... && go test ./internal/auth/... ./internal/middleware/... -count=1`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: 更新 CLAUDE.md 文档反映 auth 独立服务"
```
