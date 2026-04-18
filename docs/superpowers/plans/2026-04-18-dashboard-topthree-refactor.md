# Dashboard → TopThree 依赖重构 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dashboard 去掉 `/firstblood` 端点和 `dashboard_first_blood` 表，排行榜增加逐人逐题详情（是否解出、解题时间、一二三血），一血数据改为从 `topthree_records` 读取。

**Architecture:** Dashboard 的 `handleCorrectSubmission` 简化为仅维护 recentEvents，不再写入自己的表。`getLeaderboard` 查询增加逐题展开逻辑。`getChallengeStates` 的一血查询改为读 `topthree_records WHERE ranking=1`。

**Tech Stack:** Go, chi router, MySQL

---

### Task 1: 删除 `/firstblood` 端点和 `firstBlood` model

**Files:**
- Modify: `plugins/dashboard/dashboard.go:43-44` (删除路由注册)
- Modify: `plugins/dashboard/api.go:31-49` (删除 `getFirstBlood` handler)
- Modify: `plugins/dashboard/model.go:17-26` (删除 `firstBlood` struct)
- Modify: `plugins/dashboard/state.go:225-251` (删除 `getFirstBloodList` 函数)
- Modify: `plugins/dashboard/dashboard.go:33-34` (更新注释)

- [ ] **Step 1: 修改 `plugins/dashboard/dashboard.go` — 删除 `/firstblood` 路由，更新注释**

```go
// Register 注册仪表盘路由并订阅正确提交事件。
// 路由：
//   - GET /api/v1/dashboard/competitions/{id}/state（获取比赛状态总览，无需认证）
//
// 同时订阅 EventCorrectSubmission 事件，用于实时追踪解题动态。
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db

	// 订阅正确提交事件，用于维护最近事件列表
	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	// 注册无需认证的仪表盘路由（查看类接口）
	r.Get("/api/v1/dashboard/competitions/{id}/state", p.getState)
}
```

- [ ] **Step 2: 修改 `plugins/dashboard/api.go` — 删除 `getFirstBlood` 函数，保留 `getState`**

删除第 31-49 行的 `getFirstBlood` 函数。修改后文件只包含 `getState`：

```go
package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// getState 处理获取比赛状态总览的请求。
// 返回比赛信息、题目列表（含解题数和一血）、排行榜（含逐题详情）、统计数据和最近事件。
func (p *Plugin) getState(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	// 验证比赛 ID 格式（32 字符 UUID）
	if len(compID) != 32 {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	// 获取完整的比赛状态数据
	state, err := p.getCompetitionState(r.Context(), compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}
```

- [ ] **Step 3: 修改 `plugins/dashboard/model.go` — 删除 `firstBlood` struct**

删除第 17-26 行（`firstBlood` struct 定义）。该 struct 仅被 `getFirstBloodList` 使用，随端点一并删除。

- [ ] **Step 4: 修改 `plugins/dashboard/state.go` — 删除 `getFirstBloodList` 函数**

删除第 225-251 行（`getFirstBloodList` 函数）。

- [ ] **Step 5: 确认编译通过**

Run: `go build ./plugins/dashboard/...`
Expected: 无错误

- [ ] **Step 6: Commit**

```bash
git add plugins/dashboard/
git commit -m "refactor(dashboard): remove /firstblood endpoint and firstBlood model"
```

---

### Task 2: 修改 `leaderboardEntry` model，新增 `challengeResult`

**Files:**
- Modify: `plugins/dashboard/model.go:53-59` (修改 `leaderboardEntry`，新增 `challengeResult`)

- [ ] **Step 1: 修改 `plugins/dashboard/model.go`**

在 `firstBloodInfo` 之后、`leaderboardEntry` 之前插入 `challengeResult`，并修改 `leaderboardEntry` 增加 `Challenges` 字段：

```go
// challengeResult 表示用户在某道题目上的解题结果。
type challengeResult struct {
	ChallengeID string    `json:"challenge_id"`            // 题目 res_id
	Solved      bool      `json:"solved"`                  // 是否解出
	BloodRank   int       `json:"blood_rank,omitempty"`    // 1=一血, 2=二血, 3=三血; 0 表示非三血
	SolvedAt    time.Time `json:"solved_at,omitempty"`     // 解题时间（未解出时为零值，omitempty 不输出）
}

// leaderboardEntry 表示排行榜中的一条记录。
type leaderboardEntry struct {
	Rank        int               `json:"rank"`           // 排名
	UserID      string            `json:"user_id"`        // 用户 ID
	TotalScore  int               `json:"total_score"`    // 总得分
	LastSolveAt time.Time         `json:"last_solve_at"`  // 最后解题时间
	Challenges  []challengeResult `json:"challenges"`     // 逐题解题详情
}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./plugins/dashboard/...`
Expected: 编译可能因 `getLeaderboard` 未填充 `Challenges` 字段而有警告，但不报错（零值 `[]challengeResult` 为 nil，json 输出 null）。我们会在 Task 3 完善。

- [ ] **Step 3: Commit**

```bash
git add plugins/dashboard/model.go
git commit -m "feat(dashboard): add challengeResult to leaderboardEntry for per-user per-challenge detail"
```

---

### Task 3: 重写 `getLeaderboard` — 增加逐人逐题详情

**Files:**
- Modify: `plugins/dashboard/state.go:135-165` (重写 `getLeaderboard`)

- [ ] **Step 1: 重写 `getLeaderboard` 函数**

将原来的简单聚合查询替换为包含逐题详情的版本。新逻辑：
1. 查询比赛所有题目 ID 列表（按 res_id 排序，保证顺序一致）
2. 查询所有正确提交（user_id, challenge_id, MIN(created_at)），组装每用户每题解题状态
3. 查询 `topthree_records WHERE ranking IN (1,2,3) AND is_deleted=0`，获取 blood rank
4. 按总分降序组装最终排行榜

```go
// getLeaderboard 获取比赛的排行榜数据（含逐题详情）。
// 每个用户包含所有题目的解题状态：是否解出、解题时间、一二三血排名。
// 按总分降序，同分按最后解题时间升序。
func (p *Plugin) getLeaderboard(ctx context.Context, compID string) ([]leaderboardEntry, error) {
	// 1. 获取比赛所有题目 ID 列表（保证顺序一致）
	chalRows, err := p.db.QueryContext(ctx, `
		SELECT c.res_id
		FROM competition_challenges cc
		JOIN challenges c ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ? AND c.is_enabled = 1 AND c.is_deleted = 0
		ORDER BY c.res_id`, compID)
	if err != nil {
		return nil, err
	}
	defer chalRows.Close()

	var chalIDs []string
	for chalRows.Next() {
		var id string
		if err := chalRows.Scan(&id); err != nil {
			return nil, err
		}
		chalIDs = append(chalIDs, id)
	}

	// 2. 查询所有正确提交：每用户每题的首次解题时间
	solveRows, err := p.db.QueryContext(ctx, `
		SELECT s.user_id, s.challenge_id, MIN(s.created_at)
		FROM submissions s
		WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0
		GROUP BY s.user_id, s.challenge_id`, compID)
	if err != nil {
		return nil, err
	}
	defer solveRows.Close()

	// userSolves: user_id -> challenge_id -> solved_at
	userSolves := make(map[string]map[string]time.Time)
	var userIDs []string
	for solveRows.Next() {
		var uid, chalID string
		var solvedAt time.Time
		if err := solveRows.Scan(&uid, &chalID, &solvedAt); err != nil {
			return nil, err
		}
		if userSolves[uid] == nil {
			userSolves[uid] = make(map[string]time.Time)
			userIDs = append(userIDs, uid)
		}
		userSolves[uid][chalID] = solvedAt
	}

	// 3. 查询 topthree_records：获取所有排名 1-3 的记录
	bloodRows, err := p.db.QueryContext(ctx, `
		SELECT user_id, challenge_id, ranking
		FROM topthree_records
		WHERE competition_id = ? AND ranking IN (1,2,3) AND is_deleted = 0`, compID)
	if err != nil {
		return nil, err
	}
	defer bloodRows.Close()

	// bloodRank: "user_id:challenge_id" -> ranking
	bloodRank := make(map[string]int)
	for bloodRows.Next() {
		var uid, chalID string
		var rank int
		if err := bloodRows.Scan(&uid, &chalID, &rank); err != nil {
			return nil, err
		}
		bloodRank[uid+":"+chalID] = rank
	}

	// 4. 组装排行榜（含总分和最后解题时间排序）
	var board []leaderboardEntry
	for _, uid := range userIDs {
		solves := userSolves[uid]
		totalScore := 0
		var lastSolveAt time.Time
		challenges := make([]challengeResult, 0, len(chalIDs))

		for _, chalID := range chalIDs {
			cr := challengeResult{ChallengeID: chalID}
			if solvedAt, ok := solves[chalID]; ok {
				cr.Solved = true
				cr.SolvedAt = solvedAt
				// 查 score 需要通过 challenges 表获取
				// 总分在下面用单独查询计算更准确
				if br, hasBr := bloodRank[uid+":"+chalID]; hasBr {
					cr.BloodRank = br
				}
				if solvedAt.After(lastSolveAt) {
					lastSolveAt = solvedAt
				}
			}
			challenges = append(challenges, cr)
		}

		board = append(board, leaderboardEntry{
			UserID:      uid,
			LastSolveAt: lastSolveAt,
			Challenges:  challenges,
		})
	}

	// 5. 计算总分并排序
	scoreRows, err := p.db.QueryContext(ctx, `
		SELECT s.user_id, SUM(c.score)
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0
		GROUP BY s.user_id`, compID)
	if err != nil {
		return nil, err
	}
	defer scoreRows.Close()

	totalScores := make(map[string]int)
	for scoreRows.Next() {
		var uid string
		var score int
		if err := scoreRows.Scan(&uid, &score); err != nil {
			return nil, err
		}
		totalScores[uid] = score
	}

	for i := range board {
		board[i].TotalScore = totalScores[board[i].UserID]
	}

	// 按总分降序，同分按最后解题时间升序
	sort.Slice(board, func(i, j int) bool {
		if board[i].TotalScore != board[j].TotalScore {
			return board[i].TotalScore > board[j].TotalScore
		}
		return board[i].LastSolveAt.Before(board[j].LastSolveAt)
	})

	for i := range board {
		board[i].Rank = i + 1
	}

	if board == nil {
		board = []leaderboardEntry{}
	}
	return board, nil
}
```

需要在文件顶部添加 `"sort"` import。

- [ ] **Step 2: 确认编译通过**

Run: `go build ./plugins/dashboard/...`
Expected: 无错误

- [ ] **Step 3: Commit**

```bash
git add plugins/dashboard/state.go
git commit -m "feat(dashboard): expand leaderboard with per-user per-challenge solve status and blood rank"
```

---

### Task 4: `getChallengeStates` 一血查询改为读 `topthree_records`

**Files:**
- Modify: `plugins/dashboard/state.go:108-131` (替换 `getChallengeStates` 第三步查询)

- [ ] **Step 1: 替换 `getChallengeStates` 中的一血查询**

将第三步从 `dashboard_first_blood` 改为 `topthree_records WHERE ranking = 1`：

```go
	// 第三步：获取每道题的一血信息（从 topthree_records 读取 ranking=1）
	fbRows, err := p.db.QueryContext(ctx, `
		SELECT challenge_id, user_id, created_at
		FROM topthree_records
		WHERE competition_id = ? AND ranking = 1 AND is_deleted = 0`, compID)
	if err != nil {
		return nil, err
	}
	defer fbRows.Close()

	for fbRows.Next() {
		var chalID string
		var userID string
		var createdAt time.Time
		if err := fbRows.Scan(&chalID, &userID, &createdAt); err != nil {
			return nil, err
		}
		if cs, ok := challengeMap[chalID]; ok {
			cs.FirstBlood = &firstBloodInfo{
				UserID:    userID,
				CreatedAt: createdAt,
			}
		}
	}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./plugins/dashboard/...`
Expected: 无错误

- [ ] **Step 3: Commit**

```bash
git add plugins/dashboard/state.go
git commit -m "refactor(dashboard): read first blood from topthree_records instead of dashboard_first_blood"
```

---

### Task 5: 简化 `handleCorrectSubmission` — 不再写入 `dashboard_first_blood`

**Files:**
- Modify: `plugins/dashboard/firstblood.go` (重写 `handleCorrectSubmission`)

- [ ] **Step 1: 重写 `handleCorrectSubmission`**

改为查询 `topthree_records` 判断是否为一血，不再写入自己的表：

```go
package dashboard

import (
	"context"
	"database/sql"
	"time"

	"ad7/internal/event"
)

// handleCorrectSubmission 处理正确提交事件。
// 判断该提交是否为某道题目的前三名之一（通过查询 topthree_records），
// 并记录到内存中的最近事件列表用于实时展示。
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	ctx := context.Background()
	compID := ""
	if e.CompetitionID != nil {
		compID = *e.CompetitionID
	}

	// 查询 topthree_records 判断该用户在该题目的排名
	var rank int
	err := p.db.QueryRowContext(ctx, `
		SELECT ranking FROM topthree_records
		WHERE competition_id = ? AND challenge_id = ? AND user_id = ? AND is_deleted = 0
		LIMIT 1`, compID, e.ChallengeID, e.UserID).Scan(&rank)

	if err == nil && rank == 1 {
		// 一血事件
		p.addFirstBloodEvent(ctx, e.UserID, e.ChallengeID, time.Now())
		return
	}

	// 普通解题事件（包括 rank 2/3 也作为普通 solve 展示在事件流中）
	p.addSolveEvent(ctx, e.UserID, e.ChallengeID)
}

// addFirstBloodEvent 添加一血事件到最近事件列表。
// 查询题目标题用于展示。
func (p *Plugin) addFirstBloodEvent(ctx context.Context, userID, challengeID string, t time.Time) {
	var title string
	err := p.db.QueryRowContext(ctx, `
		SELECT title FROM challenges WHERE res_id = ? AND is_deleted = 0`, challengeID).Scan(&title)
	if err != nil {
		title = "Unknown Challenge"
	}

	p.addRecentEvent(recentEvent{
		Type:           "first_blood",
		UserID:         userID,
		ChallengeID:    challengeID,
		ChallengeTitle: title,
		CreatedAt:      t,
	})
}

// addSolveEvent 添加普通解题事件到最近事件列表。
// 查询题目标题和分数用于展示。
func (p *Plugin) addSolveEvent(ctx context.Context, userID, challengeID string) {
	var title string
	var score int
	err := p.db.QueryRowContext(ctx, `
		SELECT title, score FROM challenges WHERE res_id = ? AND is_deleted = 0`, challengeID).Scan(&title, &score)
	if err != nil {
		title = "Unknown Challenge"
		score = 0
	}

	p.addRecentEvent(recentEvent{
		Type:           "solve",
		UserID:         userID,
		ChallengeID:    challengeID,
		ChallengeTitle: title,
		Score:          score,
		CreatedAt:      time.Now(),
	})
}
```

注意：删除了 `"ad7/internal/uuid"` import（不再需要生成 res_id）。

- [ ] **Step 2: 确认编译通过**

Run: `go build ./plugins/dashboard/...`
Expected: 无错误

- [ ] **Step 3: Commit**

```bash
git add plugins/dashboard/firstblood.go
git commit -m "refactor(dashboard): simplify event handler to read blood rank from topthree_records"
```

---

### Task 6: 删除 `dashboard_first_blood` 表和 `schema.sql`

**Files:**
- Modify: `plugins/dashboard/schema.sql` (删除建表语句)
- Modify: `internal/integration/integration_test.go:136` (删除 cleanup 中的相关行)

- [ ] **Step 1: 清空 `plugins/dashboard/schema.sql`**

dashboard 不再拥有自己的表，删除文件内容或清空：

```bash
rm plugins/dashboard/schema.sql
```

- [ ] **Step 2: 确认 `integration_test.go` 的 cleanup 函数不引用 `dashboard_first_blood`**

当前 cleanup 函数（第 134-143 行）没有引用 `dashboard_first_blood`（只有 `topthree_records`），无需修改。

- [ ] **Step 3: 确认编译通过**

Run: `go build ./...`
Expected: 无错误

- [ ] **Step 4: Commit**

```bash
git add plugins/dashboard/schema.sql
git commit -m "refactor(dashboard): remove dashboard_first_blood table schema"
```

---

### Task 7: `main.go` 中确保 topthree 在 dashboard 之前注册

**Files:**
- Modify: `cmd/server/main.go:115-122` (调整插件注册顺序)

- [ ] **Step 1: 调整插件注册顺序，确保 topthree 在 dashboard 之前**

将 topthree 移到 dashboard 之前：

```go
	plugins := []plugin.Plugin{
		leaderboard.New(),  // 排行榜插件
		notification.New(), // 通知插件
		analytics.New(),    // 分析插件
		topthree.New(),     // 三血插件（必须在 dashboard 之前）
		dashboard.New(),    // 仪表盘插件
		hints.New(),        // 题目提示插件
	}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./cmd/server/...`
Expected: 无错误

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "refactor: register topthree plugin before dashboard for correct event ordering"
```

---

### Task 8: 添加集成测试 — dashboard 排行榜逐题详情

**Files:**
- Modify: `internal/integration/integration_test.go` (新增测试函数)

- [ ] **Step 1: 在 `integration_test.go` 中添加 dashboard 插件到 `TestMain`**

当前 `TestMain`（第 90 行）的 plugins 列表不包含 dashboard。添加：

```go
	plugins := []plugin.Plugin{leaderboard.New(), notification.New(), analytics.New(), hints.New(), topthree.New(), dashboard.New()}
```

并在文件顶部 import 中添加 `"ad7/plugins/dashboard"`。

- [ ] **Step 2: 在 cleanup 函数中添加 `dashboard_first_blood` 清理**

即使删除了 schema 文件，如果测试数据库中还有旧表，加上清理更安全。但当前 cleanup 已有 `topthree_records`，不需要 `dashboard_first_blood`（dashboard 不再写该表）。

如果数据库中仍存在旧 `dashboard_first_blood` 表，在 cleanup 中添加一行：

在 `testDB.Exec("DELETE FROM topthree_records")` 之前添加：
```go
testDB.Exec("DELETE FROM dashboard_first_blood")
```

（兼容旧数据库，如果表不存在会报错但不影响测试）

- [ ] **Step 3: 编写 `TestDashboardLeaderboardDetail` 测试**

在文件末尾添加：

```go
func TestDashboardLeaderboardDetail(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	u1 := makeToken("user1", "user")
	u2 := makeToken("user2", "user")
	u3 := makeToken("user3", "user")

	// 创建比赛
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"DashComp","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	// 创建 2 个题目
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"DC1","category":"web","description":"D","score":100,"flag":"flag{dc1}"}`, adminTok)
	assertStatus(t, resp, 201)
	ch1 := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"DC2","category":"pwn","description":"D","score":200,"flag":"flag{dc2}"}`, adminTok)
	assertStatus(t, resp, 201)
	ch2 := getID(t, decodeJSON(t, resp))

	// 添加题目到比赛
	doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, ch1), adminTok).Body.Close()
	doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, ch2), adminTok).Body.Close()

	// user1 解 ch1 和 ch2
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, ch1),
		`{"flag":"flag{dc1}"}`, u1).Body.Close()
	time.Sleep(10 * time.Millisecond)
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, ch2),
		`{"flag":"flag{dc2}"}`, u1).Body.Close()

	// user2 解 ch1
	time.Sleep(10 * time.Millisecond)
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, ch1),
		`{"flag":"flag{dc1}"}`, u2).Body.Close()

	// user3 解 ch1
	time.Sleep(10 * time.Millisecond)
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, ch1),
		`{"flag":"flag{dc1}"}`, u3).Body.Close()

	// 等待事件处理完成
	time.Sleep(200 * time.Millisecond)

	// 请求 dashboard state
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/dashboard/competitions/%s/state", compID), "", u1)
	assertStatus(t, resp, 200)

	var state struct {
		Leaderboard []struct {
			Rank       int `json:"rank"`
			UserID     string `json:"user_id"`
			TotalScore int `json:"total_score"`
			Challenges []struct {
				ChallengeID string `json:"challenge_id"`
				Solved      bool   `json:"solved"`
				BloodRank   int    `json:"blood_rank"`
				SolvedAt    string `json:"solved_at"`
			} `json:"challenges"`
		} `json:"leaderboard"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()

	if len(state.Leaderboard) == 0 {
		t.Fatal("expected at least 1 leaderboard entry")
	}

	// 找到 user1 的条目
	var user1Entry *struct {
		Rank       int `json:"rank"`
		UserID     string `json:"user_id"`
		TotalScore int `json:"total_score"`
		Challenges []struct {
			ChallengeID string `json:"challenge_id"`
			Solved      bool   `json:"solved"`
			BloodRank   int    `json:"blood_rank"`
			SolvedAt    string `json:"solved_at"`
		} `json:"challenges"`
	}
	for i := range state.Leaderboard {
		if state.Leaderboard[i].UserID == "user1" {
			user1Entry = &state.Leaderboard[i]
			break
		}
	}
	if user1Entry == nil {
		t.Fatal("user1 not found in leaderboard")
	}

	// user1 应排第 1（总分 300）
	if user1Entry.Rank != 1 {
		t.Errorf("expected user1 rank=1, got %d", user1Entry.Rank)
	}
	if user1Entry.TotalScore != 300 {
		t.Errorf("expected user1 total_score=300, got %d", user1Entry.TotalScore)
	}

	// user1 应解出 2 道题
	if len(user1Entry.Challenges) != 2 {
		t.Fatalf("expected 2 challenge results, got %d", len(user1Entry.Challenges))
	}

	// 验证每道题都有 solved=true 和 blood_rank=1（user1 是第一个解出的）
	for _, cr := range user1Entry.Challenges {
		if !cr.Solved {
			t.Errorf("expected solved=true for challenge %s", cr.ChallengeID)
		}
		if cr.BloodRank != 1 {
			t.Errorf("expected blood_rank=1 for challenge %s (user1 is first solver), got %d", cr.ChallengeID, cr.BloodRank)
		}
		if cr.SolvedAt == "" {
			t.Errorf("expected solved_at for challenge %s", cr.ChallengeID)
		}
	}
}
```

- [ ] **Step 4: 运行测试**

Run: `TEST_DSN="root:asfdsfedarjeiowvgfsd@tcp(192.168.5.44:3306)/ctf?parseTime=true" go test ./internal/integration/... -v -run TestDashboardLeaderboardDetail -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/integration/integration_test.go
git commit -m "test: add integration test for dashboard leaderboard per-challenge detail"
```

---

### Task 9: 运行全部集成测试确认无回归

**Files:** 无修改

- [ ] **Step 1: 运行所有集成测试**

Run: `TEST_DSN="root:asfdsfedarjeiowvgfsd@tcp(192.168.5.44:3306)/ctf?parseTime=true" go test ./internal/integration/... -v -count=1`
Expected: 全部 PASS

- [ ] **Step 2: 如果有测试失败，修复后重新运行**

常见问题：
- cleanup 中引用了不存在的表 — 移除该行
- topthree 事件处理延迟 — 增加 `time.Sleep` 时间
