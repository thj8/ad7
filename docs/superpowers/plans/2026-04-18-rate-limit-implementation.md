# Rate Limiting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement rate limiting for flag submission endpoints (3 requests per 10 seconds per user), with configurable limits and support for both IP-based and user-ID-based limiting.

**Architecture:** Use `github.com/go-chi/httprate` library, wrap it in custom middleware in `internal/middleware/`, apply to submission route in `cmd/server/main.go`.

**Tech Stack:** Go, go-chi/chi, go-chi/httprate

---

## Task 1: Add go-chi/httprate Dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the dependency**

Run:
```bash
go get github.com/go-chi/httprate
```

Expected: `go.mod` and `go.sum` are updated with the new dependency.

- [ ] **Step 2: Verify the import works**

Run:
```bash
go build ./...
```

Expected: Build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: add go-chi/httprate dependency"
```

---

## Task 2: Add Rate Limit Config Structure

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add RateLimitRule and RateLimitConfig types**

Add before the `Load()` function:

```go
import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)
```

Add these types after `JWTConfig`:

```go
// RateLimitRule defines a rate limit rule with requests per time window.
type RateLimitRule struct {
	Requests int           `yaml:"requests"` // Maximum number of requests
	Window   time.Duration `yaml:"window"`   // Time window for the limit
}

// RateLimitConfig contains rate limit configurations for different endpoints.
type RateLimitConfig struct {
	Submission RateLimitRule `yaml:"submission"` // Rate limit for flag submissions
}
```

Update the `Config` struct to include:

```go
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	DB        DBConfig        `yaml:"db"`
	JWT       JWTConfig       `yaml:"jwt"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}
```

- [ ] **Step 2: Set default values in Load() function**

In the `Load()` function, after setting default admin role, add:

```go
	// Set default rate limit for submissions: 3 requests per 10 seconds
	if cfg.RateLimit.Submission.Requests == 0 {
		cfg.RateLimit.Submission.Requests = 3
	}
	if cfg.RateLimit.Submission.Window == 0 {
		cfg.RateLimit.Submission.Window = 10 * time.Second
	}
```

- [ ] **Step 3: Run tests to verify config works**

Run:
```bash
go test ./internal/config/... -v
```

(Note: If there are no config tests, just run `go build ./...` to verify compilation)

Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add rate limit config structure"
```

---

## Task 3: Update Config Files

**Files:**
- Modify: `config.yaml`
- Modify: `config.yaml.example`

- [ ] **Step 1: Add rate limit config to config.yaml**

Add to the end of `config.yaml`:

```yaml
rate_limit:
  submission:
    requests: 3
    window: 10s
```

- [ ] **Step 2: Add rate limit config to config.yaml.example**

Add the same section to `config.yaml.example`.

- [ ] **Step 3: Commit**

```bash
git add config.yaml config.yaml.example
git commit -m "feat: add rate limit config to yaml files"
```

---

## Task 4: Create Rate Limiter Middleware (TDD - Write Test First)

**Files:**
- Create: `internal/middleware/ratelimit.go`
- Create: `internal/middleware/ratelimit_test.go`

- [ ] **Step 1: Write the test file first**

Create `internal/middleware/ratelimit_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimitByIP(t *testing.T) {
	// Create a simple handler that returns 200
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create rate limiter: 2 requests per 100ms
	limiter := LimitByIP(2, 100*time.Millisecond)
	handler := limiter(testHandler)

	// First request - should pass
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Second request - should pass
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Third request - should be rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rr.Code)
	}

	// Wait for window to reset
	time.Sleep(150 * time.Millisecond)

	// Fourth request - should pass again
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 after window reset, got %d", rr.Code)
	}
}

func TestLimitByUserID(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create rate limiter: 2 requests per 100ms
	limiter := LimitByUserID(2, 100*time.Millisecond)
	handler := limiter(testHandler)

	// Create request with user ID in context
	req := httptest.NewRequest("GET", "/", nil)
	ctx := req.Context()
	ctx = context.WithValue(ctx, CtxUserID, "user123")
	req = req.WithContext(ctx)

	// First request - should pass
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Second request - should pass
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Third request - should be rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rr.Code)
	}
}

func TestLimitByUserID_FallbackToIP(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	limiter := LimitByUserID(1, 100*time.Millisecond)
	handler := limiter(testHandler)

	// Request without user ID - should use IP
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	// First request - pass
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Second request - rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rr.Code)
	}
}
```

Also add the missing import at the top:

```go
import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/middleware/... -v
```

Expected: FAIL with "undefined: LimitByIP" and "undefined: LimitByUserID"

- [ ] **Step 3: Write the minimal implementation**

Create `internal/middleware/ratelimit.go`:

```go
// Package middleware 提供 HTTP 中间件，包括频率限制。
package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// LimitByIP 创建按 IP 地址限制的中间件。
// 参数：
//   - requests: 时间窗口内允许的最大请求数
//   - window: 时间窗口长度
//
// 返回：chi 中间件函数
func LimitByIP(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(
		requests,
		window,
		httprate.WithKeyFuncs(httprate.KeyByIP),
	)
}

// LimitByUserID 创建按用户 ID 限制的中间件。
// 用户 ID 从请求上下文的 CtxUserID 键获取（需要先经过 Authenticate 中间件）。
// 如果没有用户 ID，回退到按 IP 限制。
//
// 参数：
//   - requests: 时间窗口内允许的最大请求数
//   - window: 时间窗口长度
//
// 返回：chi 中间件函数
func LimitByUserID(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(
		requests,
		window,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			userID := UserID(r)
			if userID == "" {
				// Fallback to IP if no user ID in context
				return httprate.KeyByIP(r)
			}
			return userID, nil
		}),
	)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/middleware/... -v
```

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/middleware/ratelimit.go internal/middleware/ratelimit_test.go
git commit -m "feat: add rate limiter middleware with tests"
```

---

## Task 5: Apply Rate Limiter to Submission Route

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Update the submission route**

In `cmd/server/main.go`, find the line:

```go
r.Post("/competitions/{comp_id}/challenges/{id}/submit", submissionH.SubmitInComp)
```

Replace it with:

```go
r.With(
	middleware.LimitByUserID(
		cfg.RateLimit.Submission.Requests,
		cfg.RateLimit.Submission.Window,
	),
).Post("/competitions/{comp_id}/challenges/{id}/submit", submissionH.SubmitInComp)
```

- [ ] **Step 2: Verify the server builds**

Run:
```bash
go build ./cmd/server
```

Expected: Build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: apply rate limiter to submission endpoint"
```

---

## Task 6: Add Integration Test for Rate Limiting

**Files:**
- Modify: `internal/integration/integration_test.go`

- [ ] **Step 1: Add the integration test**

Add this test at the end of `internal/integration/integration_test.go`:

```go
func TestSubmitFlagRateLimit(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("rateuser1", "user")

	// Create competition + challenge + add to comp
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"CompRate","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"ChalRate","description":"D","score":200,"flag":"flag{rate}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	submitPath := fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID)

	// First 3 requests should succeed (rate limit is 3 per 10s)
	for i := 0; i < 3; i++ {
		resp = doRequest(t, "POST", submitPath, `{"flag":"wrong"}`, userTok)
		assertStatus(t, resp, 200)
		resp.Body.Close()
	}

	// 4th request should be rate limited (429)
	resp = doRequest(t, "POST", submitPath, `{"flag":"wrong"}`, userTok)
	assertStatus(t, resp, http.StatusTooManyRequests)
	resp.Body.Close()

	// Different user should still be able to submit
	user2Tok := makeToken("rateuser2", "user")
	resp = doRequest(t, "POST", submitPath, `{"flag":"wrong"}`, user2Tok)
	assertStatus(t, resp, 200)
	resp.Body.Close()
}
```

Also add the missing import at the top:

```go
import (
	// ... existing imports ...
	"net/http"
	// ... existing imports ...
)
```

(Note: If `net/http` is already imported, skip that part)

- [ ] **Step 2: Run the integration test**

Run:
```bash
go test ./internal/integration/... -v -run TestSubmitFlagRateLimit -count=1
```

Expected: Test passes (may require MySQL, check CLAUDE.md for details)

- [ ] **Step 3: Commit**

```bash
git add internal/integration/integration_test.go
git commit -m "test: add integration test for rate limiting"
```

---

## Task 7: Run All Tests and Verify

**Files:** None

- [ ] **Step 1: Run all tests**

Run:
```bash
go test ./...
```

Expected: All tests pass.

- [ ] **Step 2: Verify the server starts**

Run:
```bash
go run ./cmd/server -config config.yaml &
SERVER_PID=$!
sleep 1
kill $SERVER_PID
```

Expected: Server starts without errors (listening on port 8080) and shuts down cleanly.

---

## Summary

After all tasks complete:
- ✅ `go-chi/httprate` dependency added
- ✅ Config structure with defaults added
- ✅ YAML config files updated
- ✅ Middleware implemented with unit tests (TDD)
- ✅ Rate limiter applied to submission endpoint
- ✅ Integration test added
- ✅ All tests pass
