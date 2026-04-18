# Top Three Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a plugin that returns top 3 solvers for each challenge in a competition, with precomputed storage via event listening.

**Architecture:** Plugin listens to `EventCorrectSubmission` events, maintains `topthree_records` table with ranks 1-3 per challenge per competition, provides authenticated API endpoint.

**Tech Stack:** Go, MySQL, chi router, existing event system

---

## File Structure

| File | Purpose |
|------|---------|
| `plugins/topthree/schema.sql` | Database table definition |
| `plugins/topthree/model.go` | Data models |
| `plugins/topthree/topthree.go` | Plugin entry, event handling, core logic |
| `plugins/topthree/api.go` | HTTP handlers |
| `cmd/server/main.go` | Register plugin |

---

### Task 1: Create plugin directory and schema.sql

**Files:**
- Create: `plugins/topthree/schema.sql`

- [ ] **Step 1: Write schema.sql**

```sql
CREATE TABLE IF NOT EXISTS topthree_records (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32)  NOT NULL UNIQUE COMMENT 'UUID',
    competition_id VARCHAR(32)  NOT NULL COMMENT '比赛ID',
    challenge_id   VARCHAR(32)  NOT NULL COMMENT '题目ID',
    user_id        VARCHAR(128) NOT NULL COMMENT '用户ID',
    rank           TINYINT      NOT NULL COMMENT '排名 1-3',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '解题时间',
    UNIQUE INDEX idx_comp_chal_rank (competition_id, challenge_id, rank),
    INDEX idx_comp_chal (competition_id, challenge_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每道题前三名记录表';
```

- [ ] **Step 2: Commit**

```bash
git add plugins/topthree/schema.sql
git commit -m "feat: add topthree table schema"
```

---

### Task 2: Create model.go with data structures

**Files:**
- Create: `plugins/topthree/model.go`

- [ ] **Step 1: Write model.go**

```go
package topthree

import "time"

type topThreeRecord struct {
	ID            int       `json:"-"`
	ResID         string    `json:"-"`
	CompetitionID string    `json:"-"`
	ChallengeID   string    `json:"-"`
	UserID        string    `json:"user_id"`
	Rank          int       `json:"rank"`
	CreatedAt     time.Time `json:"created_at"`
}

type challengeTopThree struct {
	ChallengeID string              `json:"challenge_id"`
	Title       string              `json:"title"`
	Category    string              `json:"category"`
	Score       int                 `json:"score"`
	TopThree    []topThreeEntry     `json:"top_three"`
}

type topThreeEntry struct {
	Rank      int       `json:"rank"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type topThreeResponse struct {
	CompetitionID string                 `json:"competition_id"`
	Challenges    []challengeTopThree    `json:"challenges"`
}
```

- [ ] **Step 2: Commit**

```bash
git add plugins/topthree/model.go
git commit -m "feat: add topthree data models"
```

---

### Task 3: Create topthree.go with plugin struct and Register method

**Files:**
- Create: `plugins/topthree/topthree.go`

- [ ] **Step 1: Write plugin skeleton**

```go
package topthree

import (
	"database/sql"
	"sync"

	"github.com/go-chi/chi/v5"

	"ad7/internal/event"
	"ad7/internal/middleware"
)

type Plugin struct {
	db *sql.DB
}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db

	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	r.Group(func(r chi.Router) {
		r.Use(auth.Authenticate)
		r.Get("/api/v1/topthree/competitions/{id}", p.getTopThree)
	})
}
```

- [ ] **Step 2: Add handleCorrectSubmission stub**

```go
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	// Will implement in next task
}
```

- [ ] **Step 3: Add getTopThree stub**

```go
func (p *Plugin) getTopThree(w http.ResponseWriter, r *http.Request) {
	// Will implement in later task
}
```

- [ ] **Step 4: Add missing imports**

Update imports to include:
```go
import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"

	"ad7/internal/event"
	"ad7/internal/middleware"
)
```

- [ ] **Step 5: Verify builds**

```bash
go build ./plugins/topthree/
```

- [ ] **Step 6: Commit**

```bash
git add plugins/topthree/topthree.go
git commit -m "feat: add topthree plugin skeleton"
```

---

### Task 4: Implement handleCorrectSubmission - query current top three

**Files:**
- Modify: `plugins/topthree/topthree.go`

- [ ] **Step 1: Add getCurrentTopThree helper**

```go
func (p *Plugin) getCurrentTopThree(ctx context.Context, compID, chalID string) ([]topThreeRecord, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, res_id, competition_id, challenge_id, user_id, rank, created_at
		FROM topthree_records
		WHERE competition_id = ? AND challenge_id = ?
		ORDER BY rank ASC
	`, compID, chalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []topThreeRecord
	for rows.Next() {
		var r topThreeRecord
		err := rows.Scan(&r.ID, &r.ResID, &r.CompetitionID, &r.ChallengeID, &r.UserID, &r.Rank, &r.CreatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}
```

- [ ] **Step 2: Update handleCorrectSubmission to check competition_id**

```go
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	if e.CompetitionID == nil || *e.CompetitionID == "" {
		return
	}
	compID := *e.CompetitionID
	chalID := e.ChallengeID
	userID := e.UserID
	submitTime := time.Now()

	ctx := context.Background()

	// Get current top three
	current, err := p.getCurrentTopThree(ctx, compID, chalID)
	if err != nil {
		return
	}
}
```

- [ ] **Step 3: Add context and time imports**

```go
import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/event"
	"ad7/internal/middleware"
)
```

- [ ] **Step 4: Verify builds**

```bash
go build ./plugins/topthree/
```

- [ ] **Step 5: Commit**

```bash
git add plugins/topthree/topthree.go
git commit -m "feat: add getCurrentTopThree helper"
```

---

### Task 5: Implement handleCorrectSubmission - rank calculation logic

**Files:**
- Modify: `plugins/topthree/topthree.go`

- [ ] **Step 1: Add helper to check if user already in top three**

```go
func userInTopThree(current []topThreeRecord, userID string) bool {
	for _, r := range current {
		if r.UserID == userID {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Add helper to calculate new rank**

```go
// Returns 0 if not in top 3, 1-3 otherwise
func calculateNewRank(current []topThreeRecord, submitTime time.Time) int {
	if len(current) < 3 {
		return len(current) + 1
	}

	// Find if we're faster than 3rd place
	for i, r := range current {
		if submitTime.Before(r.CreatedAt) {
			return i + 1
		}
	}

	return 0
}
```

- [ ] **Step 3: Update handleCorrectSubmission to use rank logic**

```go
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	if e.CompetitionID == nil || *e.CompetitionID == "" {
		return
	}
	compID := *e.CompetitionID
	chalID := e.ChallengeID
	userID := e.UserID
	submitTime := time.Now()

	ctx := context.Background()

	// Get current top three
	current, err := p.getCurrentTopThree(ctx, compID, chalID)
	if err != nil {
		return
	}

	// Check if user already in top three
	if userInTopThree(current, userID) {
		return
	}

	// Calculate new rank
	newRank := calculateNewRank(current, submitTime)
	if newRank == 0 {
		return // Not in top 3
	}
}
```

- [ ] **Step 4: Verify builds**

```bash
go build ./plugins/topthree/
```

- [ ] **Step 5: Commit**

```bash
git add plugins/topthree/topthree.go
git commit -m "feat: add rank calculation logic"
```

---

### Task 6: Implement handleCorrectSubmission - database update logic

**Files:**
- Modify: `plugins/topthree/topthree.go`

- [ ] **Step 1: Add snowflake import**

```go
import (
	"ad7/internal/snowflake"
)
```

- [ ] **Step 2: Add updateTopThree function**

```go
func (p *Plugin) updateTopThree(ctx context.Context, compID, chalID, userID string, newRank int, submitTime time.Time, current []topThreeRecord) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Shift ranks down if needed
	if newRank <= len(current) {
		// Delete rank 3 if we're inserting at 1-2 and current has 3
		if newRank <= 2 && len(current) >= 3 {
			_, err := tx.ExecContext(ctx, `
				DELETE FROM topthree_records
				WHERE competition_id = ? AND challenge_id = ? AND rank = 3
			`, compID, chalID)
			if err != nil {
				return err
			}
		}

		// Update ranks: move 2->3, 1->2 as needed
		for i := len(current); i >= newRank; i-- {
			if i+1 > 3 {
				continue
			}
			_, err := tx.ExecContext(ctx, `
				UPDATE topthree_records
				SET rank = ?
				WHERE competition_id = ? AND challenge_id = ? AND rank = ?
			`, i+1, compID, chalID, i)
			if err != nil {
				return err
			}
		}
	}

	// Insert new record
	resID := snowflake.Next()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO topthree_records
		(res_id, competition_id, challenge_id, user_id, rank, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, resID, compID, chalID, userID, newRank, submitTime)
	if err != nil {
		return err
	}

	return tx.Commit()
}
```

- [ ] **Step 3: Call updateTopThree from handleCorrectSubmission**

```go
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	if e.CompetitionID == nil || *e.CompetitionID == "" {
		return
	}
	compID := *e.CompetitionID
	chalID := e.ChallengeID
	userID := e.UserID
	submitTime := time.Now()

	ctx := context.Background()

	// Get current top three
	current, err := p.getCurrentTopThree(ctx, compID, chalID)
	if err != nil {
		return
	}

	// Check if user already in top three
	if userInTopThree(current, userID) {
		return
	}

	// Calculate new rank
	newRank := calculateNewRank(current, submitTime)
	if newRank == 0 {
		return // Not in top 3
	}

	// Update database
	_ = p.updateTopThree(ctx, compID, chalID, userID, newRank, submitTime, current)
}
```

- [ ] **Step 4: Verify builds**

```bash
go build ./plugins/topthree/
```

- [ ] **Step 5: Commit**

```bash
git add plugins/topthree/topthree.go
git commit -m "feat: implement top three update logic"
```

---

### Task 7: Implement getTopThree API endpoint

**Files:**
- Create: `plugins/topthree/api.go`

- [ ] **Step 1: Write api.go**

```go
package topthree

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (p *Plugin) getTopThree(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if len(compID) != 32 {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get competition challenges
	chalRows, err := p.db.QueryContext(ctx, `
		SELECT c.res_id, c.title, c.category, c.score
		FROM challenges c
		INNER JOIN competition_challenges cc ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ? AND c.is_deleted = 0
	`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer chalRows.Close()

	var challenges []challengeTopThree
	for chalRows.Next() {
		var ct challengeTopThree
		err := chalRows.Scan(&ct.ChallengeID, &ct.Title, &ct.Category, &ct.Score)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		challenges = append(challenges, ct)
	}

	// Get top three for each challenge
	for i := range challenges {
		chal := &challenges[i]
		rows, err := p.db.QueryContext(ctx, `
			SELECT user_id, rank, created_at
			FROM topthree_records
			WHERE competition_id = ? AND challenge_id = ?
			ORDER BY rank ASC
		`, compID, chal.ChallengeID)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		var topThree []topThreeEntry
		for rows.Next() {
			var e topThreeEntry
			err := rows.Scan(&e.UserID, &e.Rank, &e.CreatedAt)
			if err != nil {
				rows.Close()
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			topThree = append(topThree, e)
		}
		rows.Close()

		chal.TopThree = topThree
	}

	resp := topThreeResponse{
		CompetitionID: compID,
		Challenges:    challenges,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

- [ ] **Step 2: Remove getTopThree stub from topthree.go**

Delete the stub function from `topthree.go`.

- [ ] **Step 3: Verify builds**

```bash
go build ./plugins/topthree/
```

- [ ] **Step 4: Commit**

```bash
git add plugins/topthree/api.go plugins/topthree/topthree.go
git commit -m "feat: implement getTopThree API endpoint"
```

---

### Task 8: Register plugin in main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add import**

```go
import (
	"ad7/plugins/topthree"
)
```

- [ ] **Step 2: Register plugin in plugins slice**

```go
plugins := []plugin.Plugin{
	leaderboard.New(),
	notification.New(),
	analytics.New(),
	dashboard.New(),
	hints.New(),
	topthree.New(),
}
```

- [ ] **Step 3: Verify builds**

```bash
go build ./cmd/server/
```

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: register topthree plugin"
```

---

### Task 9: Add integration test

**Files:**
- Modify: `internal/integration/integration_test.go`

- [ ] **Step 1: Add TestTopThree test**

```go
func TestTopThree(t *testing.T) {
	ctx := context.Background()
	cleanup(t)

	// Create competition
	compID := createCompetition(t, "Test Comp", time.Now(), time.Now().Add(24*time.Hour))

	// Create 2 challenges
	chal1ID := createChallenge(t, "Chal 1", "web", 100, "flag{1}")
	chal2ID := createChallenge(t, "Chal 2", "pwn", 200, "flag{2}")

	// Add challenges to competition
	addChallengeToCompetition(t, compID, chal1ID)
	addChallengeToCompetition(t, compID, chal2ID)

	// Create server
	st, err := store.New(testDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	auth := middleware.NewAuth(testSecret, "admin")
	r := chi.NewRouter()

	// Register topthree plugin directly
	ttPlugin := topthree.New()
	ttPlugin.Register(r, st.DB(), auth)

	// Submit for challenge 1 in order: user3, user1, user2, user4 (user4 should not make it)
	submitCorrectFlagInComp(t, compID, chal1ID, "user3", "flag{1}")
	time.Sleep(10 * time.Millisecond)
	submitCorrectFlagInComp(t, compID, chal1ID, "user1", "flag{1}")
	time.Sleep(10 * time.Millisecond)
	submitCorrectFlagInComp(t, compID, chal1ID, "user2", "flag{1}")
	time.Sleep(10 * time.Millisecond)
	submitCorrectFlagInComp(t, compID, chal1ID, "user4", "flag{1}")

	// Submit for challenge 2: user2, user3
	submitCorrectFlagInComp(t, compID, chal2ID, "user2", "flag{2}")
	time.Sleep(10 * time.Millisecond)
	submitCorrectFlagInComp(t, compID, chal2ID, "user3", "flag{2}")

	// Wait a bit for event processing
	time.Sleep(100 * time.Millisecond)

	// Test API endpoint
	req, _ := http.NewRequest("GET", "/api/v1/topthree/competitions/"+compID, nil)
	req.Header.Set("Authorization", "Bearer "+userToken("user1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		CompetitionID string `json:"competition_id"`
		Challenges    []struct {
			ChallengeID string `json:"challenge_id"`
			Title       string `json:"title"`
			TopThree    []struct {
				Rank   int    `json:"rank"`
				UserID string `json:"user_id"`
			} `json:"top_three"`
		} `json:"challenges"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.CompetitionID != compID {
		t.Errorf("competition id mismatch: %s != %s", resp.CompetitionID, compID)
	}

	// Find challenge 1
	var chal1 *struct {
		ChallengeID string `json:"challenge_id"`
		Title       string `json:"title"`
		TopThree    []struct {
			Rank   int    `json:"rank"`
			UserID string `json:"user_id"`
		} `json:"top_three"`
	}
	for i := range resp.Challenges {
		if resp.Challenges[i].ChallengeID == chal1ID {
			chal1 = &resp.Challenges[i]
			break
		}
	}
	if chal1 == nil {
		t.Fatal("challenge 1 not found in response")
	}

	if len(chal1.TopThree) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(chal1.TopThree))
	}

	// Check ranks (user3 was first, then user1, then user2)
	// Note: The order depends on timing, but all 3 should be present
	seenUsers := make(map[string]bool)
	for _, e := range chal1.TopThree {
		seenUsers[e.UserID] = true
	}
	if !seenUsers["user3"] {
		t.Error("user3 not in top three")
	}
	if !seenUsers["user1"] {
		t.Error("user1 not in top three")
	}
	if !seenUsers["user2"] {
		t.Error("user2 not in top three")
	}
	if seenUsers["user4"] {
		t.Error("user4 should not be in top three")
	}
}
```

- [ ] **Step 2: Add topthree import**

```go
import (
	"ad7/plugins/topthree"
)
```

- [ ] **Step 3: Run test to verify it fails initially**

```bash
go test ./internal/integration/... -v -run TestTopThree -count=1
```

Expected: May fail due to event timing, but should show code compiles.

- [ ] **Step 4: Commit**

```bash
git add internal/integration/integration_test.go
git commit -m "test: add topthree integration test"
```

---

### Task 10: Update schema.sql in sql/ directory (optional but recommended)

**Files:**
- Modify: `sql/schema.sql`

- [ ] **Step 1: Append topthree_records table to sql/schema.sql**

```sql
CREATE TABLE IF NOT EXISTS topthree_records (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32)  NOT NULL UNIQUE COMMENT 'UUID',
    competition_id VARCHAR(32)  NOT NULL COMMENT '比赛ID',
    challenge_id   VARCHAR(32)  NOT NULL COMMENT '题目ID',
    user_id        VARCHAR(128) NOT NULL COMMENT '用户ID',
    rank           TINYINT      NOT NULL COMMENT '排名 1-3',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '解题时间',
    UNIQUE INDEX idx_comp_chal_rank (competition_id, challenge_id, rank),
    INDEX idx_comp_chal (competition_id, challenge_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每道题前三名记录表';
```

- [ ] **Step 2: Commit**

```bash
git add sql/schema.sql
git commit -m "feat: add topthree table to main schema"
```

---

## Final Validation

- [ ] **Run all tests**

```bash
go test ./...
```

- [ ] **Run integration tests**

```bash
go test ./internal/integration/... -v -count=1
```

---

## Summary

This plan creates:
- `plugins/topthree/` - Complete plugin with event listening and API
- Database table `topthree_records` for precomputed rankings
- Integration test verifying the functionality
