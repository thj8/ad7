// Package leaderboard 实现比赛排行榜插件。
// 按比赛维度统计用户的总得分和最后解题时间，
// 排行规则：总分降序，同分按最后正确提交时间升序（越早越好）。
// 每个用户包含逐题详情：是否解出、解题时间、一二三血排名。
package leaderboard

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

// Plugin 是排行榜插件，持有数据库连接。
type Plugin struct{ db *sql.DB }

// New 创建排行榜插件实例。
func New() *Plugin { return &Plugin{} }

// Register 注册排行榜的路由。
// 路由：GET /api/v1/competitions/{id}/leaderboard（需要认证）
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/leaderboard", p.listByComp)
}

// challengeResult 表示用户在某道题目上的解题结果。
type challengeResult struct {
	ChallengeID string    `json:"challenge_id"`         // 题目 res_id
	Solved      bool      `json:"solved"`               // 是否解出
	BloodRank   int       `json:"blood_rank,omitempty"` // 1=一血, 2=二血, 3=三血; 普通解题不输出
	SolvedAt    time.Time `json:"solved_at,omitempty"`  // 解题时间
}

// entry 是排行榜中的一条记录，表示一个用户的排名信息。
type entry struct {
	Rank          int               `json:"rank"`            // 排名（从 1 开始）
	UserID        string            `json:"user_id"`         // 用户 ID
	TotalScore    int               `json:"total_score"`     // 总得分
	LastSolveTime time.Time         `json:"last_solve_time"` // 最后一次正确提交的时间
	Challenges    []challengeResult `json:"challenges"`      // 逐题解题详情
}

// listByComp 处理获取比赛排行榜的请求。
// 返回每个用户的排名、总分、最后解题时间，以及每道题的解题状态和一二三血信息。
func (p *Plugin) listByComp(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	ctx := r.Context()

	// 1. 获取比赛所有题目 ID
	chalRows, err := p.db.QueryContext(ctx, `
		SELECT c.res_id
		FROM competition_challenges cc
		JOIN challenges c ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ? AND c.is_enabled = 1 AND c.is_deleted = 0
		ORDER BY c.res_id`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer chalRows.Close()

	var chalIDs []string
	for chalRows.Next() {
		var id string
		if err := chalRows.Scan(&id); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		chalIDs = append(chalIDs, id)
	}

	// 2. 查询所有正确提交
	solveRows, err := p.db.QueryContext(ctx, `
		SELECT s.user_id, s.challenge_id, MIN(s.created_at)
		FROM submissions s
		WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0
		GROUP BY s.user_id, s.challenge_id`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer solveRows.Close()

	userSolves := make(map[string]map[string]time.Time)
	var userIDs []string
	for solveRows.Next() {
		var uid, chalID string
		var solvedAt time.Time
		if err := solveRows.Scan(&uid, &chalID, &solvedAt); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		if userSolves[uid] == nil {
			userSolves[uid] = make(map[string]time.Time)
			userIDs = append(userIDs, uid)
		}
		userSolves[uid][chalID] = solvedAt
	}

	// 3. 查询 topthree_records 获取一二三血排名
	bloodRows, err := p.db.QueryContext(ctx, `
		SELECT user_id, challenge_id, ranking
		FROM topthree_records
		WHERE competition_id = ? AND ranking IN (1,2,3) AND is_deleted = 0`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer bloodRows.Close()

	bloodRank := make(map[string]int)
	for bloodRows.Next() {
		var uid, chalID string
		var rank int
		if err := bloodRows.Scan(&uid, &chalID, &rank); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		bloodRank[uid+":"+chalID] = rank
	}

	// 4. 计算总分
	scoreRows, err := p.db.QueryContext(ctx, `
		SELECT s.user_id, SUM(c.score)
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0
		GROUP BY s.user_id`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer scoreRows.Close()

	totalScores := make(map[string]int)
	for scoreRows.Next() {
		var uid string
		var score int
		if err := scoreRows.Scan(&uid, &score); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		totalScores[uid] = score
	}

	// 5. 组装排行榜
	var board []entry
	for _, uid := range userIDs {
		solves := userSolves[uid]
		var lastSolveAt time.Time
		challenges := make([]challengeResult, 0, len(chalIDs))

		for _, chalID := range chalIDs {
			cr := challengeResult{ChallengeID: chalID}
			if solvedAt, ok := solves[chalID]; ok {
				cr.Solved = true
				cr.SolvedAt = solvedAt
				if br, hasBr := bloodRank[uid+":"+chalID]; hasBr {
					cr.BloodRank = br
				}
				if solvedAt.After(lastSolveAt) {
					lastSolveAt = solvedAt
				}
			}
			challenges = append(challenges, cr)
		}

		board = append(board, entry{
			UserID:        uid,
			TotalScore:    totalScores[uid],
			LastSolveTime: lastSolveAt,
			Challenges:    challenges,
		})
	}

	sort.Slice(board, func(i, j int) bool {
		if board[i].TotalScore != board[j].TotalScore {
			return board[i].TotalScore > board[j].TotalScore
		}
		return board[i].LastSolveTime.Before(board[j].LastSolveTime)
	})

	for i := range board {
		board[i].Rank = i + 1
	}

	if board == nil {
		board = []entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"leaderboard": board})
}
