# Competition Start/End Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add automatic (lazy) and manual (admin API) start/end operations for competitions.

**Architecture:** Store layer adds `SetActive` to toggle `is_active` by `res_id`. Service layer adds `StartCompetition`/`EndCompetition` for manual control and `syncStatus` for lazy auto-evaluation. Handler layer exposes `POST /admin/competitions/{id}/start` and `/end`.

**Tech Stack:** Go, chi router, MySQL, `log/slog`

---

### Task 1: Store Layer — Add SetActive

**Files:**
- Modify: `internal/store/store.go:59-92` (CompetitionStore interface)
- Modify: `internal/store/mysql.go:300+` (implementation)

- [ ] **Step 1: Add SetActive to CompetitionStore interface**

在 `internal/store/store.go` 的 `CompetitionStore` 接口中，在 `ListCompChallenges` 之后添加：

```go
	// SetActive 设置比赛的 is_active 状态。
	// 通过 res_id 定位，仅更新未删除的比赛。
	SetActive(ctx context.Context, resID string, active bool) error
```

- [ ] **Step 2: Implement SetActive in mysql.go**

在 `internal/store/mysql.go` 的 `ListCompChallenges` 方法之后添加：

```go
// SetActive 根据 res_id 设置比赛的 is_active 状态。
// WHERE 条件包含 is_deleted = 0，不会修改已删除的比赛。
func (s *Store) SetActive(ctx context.Context, resID string, active bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE competitions SET is_active = ? WHERE res_id = ? AND is_deleted = 0`,
		active, resID)
	return err
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/store/store.go internal/store/mysql.go
git commit -m "feat(store): add SetActive method for competition status"
```

---

### Task 2: Service Layer — ErrConflict, StartCompetition, EndCompetition

**Files:**
- Modify: `internal/service/competition.go`

- [ ] **Step 1: Add ErrConflict and imports**

在 `internal/service/competition.go` 的 import 中添加 `"log/slog"` 和 `"time"`：

```go
import (
	"context"
	"errors"
	"log/slog"
	"time"

	"ad7/internal/model"
	"ad7/internal/store"
)
```

在文件中（`ErrNotFound` 可见的位置，但它在 `challenge.go` 中已定义为包级变量）添加 `ErrConflict`：

在 `CompetitionService` 结构体定义之前添加：

```go
// ErrConflict 表示操作冲突（如重复开始/结束比赛）。
var ErrConflict = errors.New("conflict")
```

- [ ] **Step 2: Add StartCompetition method**

在 `ListChallenges` 方法之后添加：

```go
// StartCompetition 手动开始比赛，设置 is_active = true。
// 如果比赛已激活返回 ErrConflict，不存在返回 ErrNotFound。
func (s *CompetitionService) StartCompetition(ctx context.Context, resID string) (*model.Competition, error) {
	c, err := s.Get(ctx, resID)
	if err != nil {
		return nil, err
	}
	if c.IsActive {
		return nil, ErrConflict
	}
	if err := s.store.SetActive(ctx, resID, true); err != nil {
		return nil, err
	}
	c.IsActive = true
	slog.Info("competition started", "competition_id", resID)
	return c, nil
}
```

- [ ] **Step 3: Add EndCompetition method**

```go
// EndCompetition 手动结束比赛，设置 is_active = false。
// 如果比赛已结束返回 ErrConflict，不存在返回 ErrNotFound。
func (s *CompetitionService) EndCompetition(ctx context.Context, resID string) (*model.Competition, error) {
	c, err := s.Get(ctx, resID)
	if err != nil {
		return nil, err
	}
	if !c.IsActive {
		return nil, ErrConflict
	}
	if err := s.store.SetActive(ctx, resID, false); err != nil {
		return nil, err
	}
	c.IsActive = false
	slog.Info("competition ended", "competition_id", resID)
	return c, nil
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add internal/service/competition.go
git commit -m "feat(service): add StartCompetition, EndCompetition methods"
```

---

### Task 3: Service Layer — syncStatus (Lazy Auto-Evaluation)

**Files:**
- Modify: `internal/service/competition.go`

- [ ] **Step 1: Add syncStatus private method**

在 `EndCompetition` 之后、文件末尾之前添加：

```go
// syncStatus 检查比赛时间并自动更新状态。
// 激活条件：start_time <= now && end_time > now && is_active == false
// 结束条件：end_time <= now && is_active == true
// 仅在状态实际变更时才写库，修改传入的 Competition 的 IsActive 字段。
func (s *CompetitionService) syncStatus(ctx context.Context, c *model.Competition) {
	now := time.Now()
	// 自动激活
	if !c.IsActive && !now.Before(c.StartTime) && now.Before(c.EndTime) {
		c.IsActive = true
		_ = s.store.SetActive(ctx, c.ResID, true)
		slog.Info("competition auto-activated", "competition_id", c.ResID)
	}
	// 自动结束
	if c.IsActive && !now.Before(c.EndTime) {
		c.IsActive = false
		_ = s.store.SetActive(ctx, c.ResID, false)
		slog.Info("competition auto-ended", "competition_id", c.ResID)
	}
}
```

- [ ] **Step 2: Wire syncStatus into Get**

修改 `Get` 方法，在返回之前调用 `syncStatus`：

```go
func (s *CompetitionService) Get(ctx context.Context, resID string) (*model.Competition, error) {
	c, err := s.store.GetCompetitionByID(ctx, resID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	s.syncStatus(ctx, c)
	return c, nil
}
```

- [ ] **Step 3: Wire syncStatus into ListActive**

修改 `ListActive` 方法，对每个结果检查并过滤已过期的比赛：

```go
func (s *CompetitionService) ListActive(ctx context.Context) ([]model.Competition, error) {
	cs, err := s.store.ListActiveCompetitions(ctx)
	if err != nil {
		return nil, err
	}
	var active []model.Competition
	for i := range cs {
		s.syncStatus(ctx, &cs[i])
		if cs[i].IsActive {
			active = append(active, cs[i])
		}
	}
	return active, nil
}
```

- [ ] **Step 4: Wire syncStatus into List**

修改 `List` 方法（管理员列表，不过滤只更新状态）：

```go
func (s *CompetitionService) List(ctx context.Context) ([]model.Competition, error) {
	cs, err := s.store.ListCompetitions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range cs {
		s.syncStatus(ctx, &cs[i])
	}
	return cs, nil
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add internal/service/competition.go
git commit -m "feat(service): add syncStatus for lazy auto start/end"
```

---

### Task 4: Handler Layer — Start and End Handlers

**Files:**
- Modify: `internal/handler/competition.go`

- [ ] **Step 1: Add Start handler method**

在 `RemoveChallenge` 方法之后添加：

```go
// Start 处理 POST /api/v1/admin/competitions/{id}/start 请求（管理员）。
// 手动激活指定比赛。返回更新后的比赛信息。
// 如果比赛已激活返回 409，不存在返回 404。
func (h *CompetitionHandler) Start(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.StartCompetition(r.Context(), id)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err == service.ErrConflict {
		writeError(w, http.StatusConflict, "competition already started")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}
```

- [ ] **Step 2: Add End handler method**

```go
// End 处理 POST /api/v1/admin/competitions/{id}/end 请求（管理员）。
// 手动结束指定比赛。返回更新后的比赛信息。
// 如果比赛已结束返回 409，不存在返回 404。
func (h *CompetitionHandler) End(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.EndCompetition(r.Context(), id)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err == service.ErrConflict {
		writeError(w, http.StatusConflict, "competition already ended")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/handler/competition.go
git commit -m "feat(handler): add Start and End handlers for competition"
```

---

### Task 5: Router — Register New Routes

**Files:**
- Modify: `internal/router/competitions.go`
- Modify: `internal/integration/integration_test.go:91-103`

- [ ] **Step 1: Add routes to router/competitions.go**

在 admin 路由组的 `r.Delete("/competitions/{id}/challenges/{challenge_id}", ...)` 之后添加：

```go
		r.Post("/competitions/{id}/start", deps.CompetitionH.Start)
		r.Post("/competitions/{id}/end", deps.CompetitionH.End)
```

- [ ] **Step 2: Add routes to integration test TestMain**

在 `internal/integration/integration_test.go` 的 admin 路由组中，在 `r.Delete("/competitions/{id}/challenges/{challenge_id}", compH.RemoveChallenge)` 之后添加同样的两行：

```go
			r.Post("/competitions/{id}/start", compH.Start)
			r.Post("/competitions/{id}/end", compH.End)
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/router/competitions.go internal/integration/integration_test.go
git commit -m "feat(router): register start/end competition routes"
```

---

### Task 6: Integration Tests — Manual Start/End + Auto Status

**Files:**
- Modify: `internal/integration/integration_test.go`

- [ ] **Step 1: Write TestCompetitionStartEnd**

在 `internal/integration/integration_test.go` 的最后一个测试函数之后添加：

```go
func TestCompetitionStartEnd(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")

	// 创建比赛（默认 is_active=true）
	startTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	endTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"Test Comp","description":"desc","start_time":"`+startTime+`","end_time":"`+endTime+`"}`,
		adminTok)
	assertStatus(t, resp, http.StatusCreated)
	compID := getID(t, decodeJSON(t, resp))

	// 1. 手动结束比赛
	resp = doRequest(t, "POST", "/api/v1/admin/competitions/"+compID+"/end", "", adminTok)
	assertStatus(t, resp, http.StatusOK)
	body := decodeJSON(t, resp)
	if body["is_active"] != false {
		t.Fatalf("expected is_active=false after end, got %v", body["is_active"])
	}

	// 2. 重复结束 → 409
	resp = doRequest(t, "POST", "/api/v1/admin/competitions/"+compID+"/end", "", adminTok)
	assertStatus(t, resp, http.StatusConflict)

	// 3. 手动开始比赛
	resp = doRequest(t, "POST", "/api/v1/admin/competitions/"+compID+"/start", "", adminTok)
	assertStatus(t, resp, http.StatusOK)
	body = decodeJSON(t, resp)
	if body["is_active"] != true {
		t.Fatalf("expected is_active=true after start, got %v", body["is_active"])
	}

	// 4. 重复开始 → 409
	resp = doRequest(t, "POST", "/api/v1/admin/competitions/"+compID+"/start", "", adminTok)
	assertStatus(t, resp, http.StatusConflict)

	// 5. 不存在的比赛 → 404
	resp = doRequest(t, "POST", "/api/v1/admin/competitions/00000000000000000000000000000000/start", "", adminTok)
	assertStatus(t, resp, http.StatusNotFound)

	// 6. 非 admin → 403
	userTok := makeToken("user1", "user")
	resp = doRequest(t, "POST", "/api/v1/admin/competitions/"+compID+"/start", "", userTok)
	assertStatus(t, resp, http.StatusForbidden)
}
```

- [ ] **Step 2: Write TestCompetitionAutoStatus**

在同一文件中添加惰性自动状态检查测试：

```go
func TestCompetitionAutoStatus(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// --- 自动激活测试 ---
	// 创建一个 start_time 在过去、end_time 在未来的比赛
	start1 := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	end1 := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"Auto Activate","description":"","start_time":"`+start1+`","end_time":"`+end1+`"}`,
		adminTok)
	assertStatus(t, resp, http.StatusCreated)
	comp1ID := getID(t, decodeJSON(t, resp))

	// 手动结束比赛使其 is_active=false
	resp = doRequest(t, "POST", "/api/v1/admin/competitions/"+comp1ID+"/end", "", adminTok)
	assertStatus(t, resp, http.StatusOK)

	// 通过 Get 触发 syncStatus，应自动激活
	resp = doRequest(t, "GET", "/api/v1/competitions/"+comp1ID, "", userTok)
	assertStatus(t, resp, http.StatusOK)
	body := decodeJSON(t, resp)
	if body["is_active"] != true {
		t.Fatalf("expected auto-activation (is_active=true), got %v", body["is_active"])
	}

	// --- 自动结束测试 ---
	// 创建一个 end_time 在过去的比赛
	start2 := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
	end2 := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	resp = doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"Auto End","description":"","start_time":"`+start2+`","end_time":"`+end2+`"}`,
		adminTok)
	assertStatus(t, resp, http.StatusCreated)
	comp2ID := getID(t, decodeJSON(t, resp))

	// 新建比赛默认 is_active=true，但 end_time 已过
	// 通过 Get 触发 syncStatus，应自动结束
	resp = doRequest(t, "GET", "/api/v1/competitions/"+comp2ID, "", userTok)
	assertStatus(t, resp, http.StatusOK)
	body = decodeJSON(t, resp)
	if body["is_active"] != false {
		t.Fatalf("expected auto-ending (is_active=false), got %v", body["is_active"])
	}

	// --- ListActive 过滤测试 ---
	// comp2 已自动结束，不应出现在 ListActive 中
	resp = doRequest(t, "GET", "/api/v1/competitions", "", userTok)
	assertStatus(t, resp, http.StatusOK)
	listBody := decodeJSON(t, resp)
	comps := listBody["competitions"].([]any)
	for _, c := range comps {
		m := c.(map[string]any)
		if m["id"] == comp2ID {
			t.Fatal("auto-ended competition should not appear in ListActive")
		}
	}
}
```

- [ ] **Step 3: Run integration tests**

Run: `go test ./internal/integration/... -v -run "TestCompetitionStartEnd|TestCompetitionAutoStatus" -count=1`
Expected: all PASS

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/integration/integration_test.go
git commit -m "test: add integration tests for competition start/end and auto-status"
```
