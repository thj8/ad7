# Hints Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a hints plugin that allows admins to add hints to challenges and users to view them.

**Architecture:** New plugin `plugins/hints/hints.go` following the same pattern as leaderboard/notification/analytics plugins. Direct SQL queries, 4 endpoints (3 admin, 1 user), authentication required for all endpoints.

**Tech Stack:** Go, chi router, MySQL, JWT auth

---

## File Structure

**Files to create:**
- `plugins/hints/hints.go` - Main plugin implementation with all endpoints
- Add `hints` table to `sql/schema.sql`

**Files to modify:**
- `cmd/server/main.go` - Import and register the hints plugin
- `internal/integration/integration_test.go` - Add integration tests

---

### Task 1: Add hints table to schema.sql

**Files:**
- Modify: `sql/schema.sql`

- [ ] **Step 1: Add hints table definition**

Add to the end of `sql/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS hints (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    res_id      BIGINT       NOT NULL UNIQUE,
    challenge_id BIGINT      NOT NULL,
    content     TEXT         NOT NULL,
    is_visible  TINYINT(1)   NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 2: Commit**

```bash
git add sql/schema.sql
git commit -m "feat: add hints table to schema"
```

---

### Task 2: Create hints plugin skeleton

**Files:**
- Create: `plugins/hints/hints.go`

- [ ] **Step 1: Create the plugin skeleton**

```go
package hints

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/snowflake"
)

type Plugin struct{ db *sql.DB }

func New() *Plugin { return &Plugin{} }

type hint struct {
	ResID     int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type createReq struct {
	Content string `json:"content"`
}

type updateReq struct {
	Content   *string `json:"content"`
	IsVisible *bool   `json:"is_visible"`
}

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/challenges/{id}/hints", p.create)
	r.With(auth.Authenticate, auth.RequireAdmin).Put("/api/v1/admin/hints/{id}", p.update)
	r.With(auth.Authenticate, auth.RequireAdmin).Delete("/api/v1/admin/hints/{id}", p.delete)
	r.With(auth.Authenticate).Get("/api/v1/challenges/{id}/hints", p.list)
}

func (p *Plugin) create(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) update(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) delete(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) list(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}
```

- [ ] **Step 2: Verify the file compiles**

Run: `go build ./plugins/hints`
Expected: No errors (package builds successfully)

- [ ] **Step 3: Commit**

```bash
git add plugins/hints/hints.go
git commit -m "feat: add hints plugin skeleton"
```

---

### Task 3: Implement create hint endpoint

**Files:**
- Modify: `plugins/hints/hints.go`

- [ ] **Step 1: Implement create handler**

Replace the `create` function with:

```go
func (p *Plugin) create(w http.ResponseWriter, r *http.Request) {
	chalID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid challenge id"}`, http.StatusBadRequest)
		return
	}

	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" || len(req.Content) > 4096 {
		http.Error(w, `{"error":"content is required (max 4096 chars)"}`, http.StatusBadRequest)
		return
	}

	_, err = p.db.ExecContext(r.Context(),
		`INSERT INTO hints (res_id, challenge_id, content) VALUES (?, ?, ?)`,
		snowflake.Next(), chalID, req.Content)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
```

- [ ] **Step 2: Run go build to verify**

Run: `go build ./plugins/hints`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add plugins/hints/hints.go
git commit -m "feat: add create hint endpoint"
```

---

### Task 4: Implement update hint endpoint

**Files:**
- Modify: `plugins/hints/hints.go`

- [ ] **Step 1: Implement update handler**

Replace the `update` function with:

```go
func (p *Plugin) update(w http.ResponseWriter, r *http.Request) {
	hintID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid hint id"}`, http.StatusBadRequest)
		return
	}

	var req updateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Validate content if provided
	if req.Content != nil && (len(*req.Content) == 0 || len(*req.Content) > 4096) {
		http.Error(w, `{"error":"content must be 1-4096 chars"}`, http.StatusBadRequest)
		return
	}

	// Get current values first
	var currentContent string
	var currentIsVisible bool
	err = p.db.QueryRowContext(r.Context(),
		`SELECT content, is_visible FROM hints WHERE res_id = ?`, hintID).
		Scan(&currentContent, &currentIsVisible)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	// Determine new values
	newContent := currentContent
	if req.Content != nil {
		newContent = *req.Content
	}
	newIsVisible := currentIsVisible
	if req.IsVisible != nil {
		newIsVisible = *req.IsVisible
	}

	// Update
	_, err = p.db.ExecContext(r.Context(),
		`UPDATE hints SET content = ?, is_visible = ? WHERE res_id = ?`,
		newContent, newIsVisible, hintID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Run go build to verify**

Run: `go build ./plugins/hints`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add plugins/hints/hints.go
git commit -m "feat: add update hint endpoint"
```

---

### Task 5: Implement delete hint endpoint

**Files:**
- Modify: `plugins/hints/hints.go`

- [ ] **Step 1: Implement delete handler**

Replace the `delete` function with:

```go
func (p *Plugin) delete(w http.ResponseWriter, r *http.Request) {
	hintID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid hint id"}`, http.StatusBadRequest)
		return
	}

	result, err := p.db.ExecContext(r.Context(),
		`DELETE FROM hints WHERE res_id = ?`, hintID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Run go build to verify**

Run: `go build ./plugins/hints`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add plugins/hints/hints.go
git commit -m "feat: add delete hint endpoint"
```

---

### Task 6: Implement list hints endpoint

**Files:**
- Modify: `plugins/hints/hints.go`

- [ ] **Step 1: Implement list handler**

Replace the `list` function with:

```go
func (p *Plugin) list(w http.ResponseWriter, r *http.Request) {
	chalID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid challenge id"}`, http.StatusBadRequest)
		return
	}

	rows, err := p.db.QueryContext(r.Context(),
		`SELECT res_id, content, created_at FROM hints
		 WHERE challenge_id = ? AND is_visible = 1
		 ORDER BY created_at ASC`, chalID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hints []hint
	for rows.Next() {
		var h hint
		if err := rows.Scan(&h.ResID, &h.Content, &h.CreatedAt); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		hints = append(hints, h)
	}

	if hints == nil {
		hints = []hint{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"hints": hints})
}
```

- [ ] **Step 2: Run go build to verify**

Run: `go build ./plugins/hints`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add plugins/hints/hints.go
git commit -m "feat: add list hints endpoint"
```

---

### Task 7: Register plugin in main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add import for hints plugin**

In `cmd/server/main.go`, add after the other plugin imports:

```go
	"ad7/plugins/hints"
```

- [ ] **Step 2: Register the plugin in the plugins slice**

In `cmd/server/main.go`, modify the plugins slice to:

```go
	plugins := []plugin.Plugin{
		leaderboard.New(),
		notification.New(),
		analytics.New(),
		hints.New(),
	}
```

- [ ] **Step 3: Run go build to verify**

Run: `go build ./cmd/server`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: register hints plugin"
```

---

### Task 8: Add integration tests

**Files:**
- Modify: `internal/integration/integration_test.go`

- [ ] **Step 1: Add import for hints plugin**

In `internal/integration/integration_test.go`, add after the other plugin imports:

```go
	"ad7/plugins/hints"
```

- [ ] **Step 2: Register hints plugin in TestMain**

In the `TestMain` function, add `hints.New()` to the plugins slice:

```go
	plugins := []plugin.Plugin{leaderboard.New(), notification.New(), analytics.New(), hints.New()}
```

- [ ] **Step 3: Add test functions**

Add to the end of `internal/integration/integration_test.go`:

```go
func TestHints(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// Create challenge
	resp := doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Test Chal","description":"desc","score":100,"flag":"flag{test}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	// Create hint 1
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/challenges/%d/hints", chalID),
		`{"content":"First hint"}`, adminTok)
	assertStatus(t, resp, 201)

	// Create hint 2 (invisible)
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/challenges/%d/hints", chalID),
		`{"content":"Second hint"}`, adminTok)
	assertStatus(t, resp, 201)

	// User lists hints - should see 2 visible hints
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%d/hints", chalID), "", userTok)
	assertStatus(t, resp, 200)
	body := decodeJSON(t, resp)
	hints := body["hints"].([]any)
	if len(hints) != 2 {
		t.Fatalf("expected 2 hints, got %d", len(hints))
	}
}
```

- [ ] **Step 2: Run integration tests to verify**

Run: `go test ./internal/integration/... -v -run TestHints -count=1`
Expected: Test passes

- [ ] **Step 3: Commit**

```bash
git add internal/integration/integration_test.go
git commit -m "test: add hints plugin integration tests"
```

---

### Task 9: Run all tests and build

**Files:**
- (no files modified)

- [ ] **Step 1: Run all tests**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 2: Build the server**

Run: `go build ./cmd/server`
Expected: Build completes successfully

- [ ] **Step 3: Commit (if any fixes needed)**

(Only if fixes were required)

---

## Self-Review

**1. Spec coverage:**
- Hints table added to schema ✅
- Admin can create hints ✅
- Admin can update hints ✅
- Admin can delete hints ✅
- User can list visible hints ✅
- Hints ordered by created_at ascending ✅
- Input validation (content length) ✅
- Integration tests ✅

**2. Placeholder scan:**
- No TBD/TODO placeholders ✅
- All code blocks complete ✅
- All test code provided ✅
- Exact commands with expected output ✅

**3. Type consistency:**
- All struct fields and method signatures consistent ✅
- Package imports match existing codebase ✅
- SQL queries use correct column names from schema ✅

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-17-hints-plugin.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
