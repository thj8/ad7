// Package leaderboard 实现比赛排行榜插件。
// 按比赛维度统计用户的总得分和最后解题时间，
// 排行规则：总分降序，同分按最后正确提交时间升序（越早越好）。
// 每个用户包含逐题详情：是否解出、解题时间、一二三血排名。
package leaderboard

import (
	"database/sql"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/pluginutil"
	"ad7/plugins/topthree"
)

// Plugin 是排行榜插件，持有数据库连接和依赖。
type Plugin struct {
	db        *sql.DB
	topThree  topthree.TopThreeProvider
}

// New 创建排行榜插件实例。
func New() *Plugin { return &Plugin{} }

// Name 返回插件名称
func (p *Plugin) Name() string {
	return plugin.NameLeaderboard
}

// Register 注册排行榜的路由。
// 路由：GET /api/v1/competitions/{id}/leaderboard（需要认证）
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db

	// 从依赖中获取 topthree 插件的 TopThreeProvider 接口
	if topThreePlugin, ok := deps[plugin.NameTopThree]; ok {
		if provider, ok := topThreePlugin.(topthree.TopThreeProvider); ok {
			p.topThree = provider
		}
	}

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
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}
	ctx := r.Context()

	// 1. 获取比赛所有题目 ID（通过共享查询函数获取完整信息，再提取 ID）
	challenges, err := pluginutil.GetCompChallenges(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	chalIDs := make([]string, 0, len(challenges))
	for _, c := range challenges {
		chalIDs = append(chalIDs, c.ResID)
	}

	// 2. 查询所有正确提交
	solves, err := pluginutil.GetCorrectSubmissions(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	userSolves := make(map[string]map[string]time.Time)
	var userIDs []string
	for _, fs := range solves {
		if userSolves[fs.UserID] == nil {
			userSolves[fs.UserID] = make(map[string]time.Time)
			userIDs = append(userIDs, fs.UserID)
		}
		userSolves[fs.UserID][fs.ChallengeID] = fs.SolvedAt
	}

	// 3. 通过 TopThreeProvider 接口获取一二三血排名
	bloodRank := make(map[string]int)
	if p.topThree != nil {
		topThreeMap, err := p.topThree.GetCompTopThree(ctx, compID)
		if err == nil {
			for chalID, entry := range topThreeMap {
				if entry.FirstBlood != "" {
					bloodRank[entry.FirstBlood+":"+chalID] = 1
				}
				if entry.SecondBlood != "" {
					bloodRank[entry.SecondBlood+":"+chalID] = 2
				}
				if entry.ThirdBlood != "" {
					bloodRank[entry.ThirdBlood+":"+chalID] = 3
				}
			}
		}
	}

	// 4. 计算总分
	totalScores, err := pluginutil.GetUserScores(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	// 5. 组装排行榜
	var board []entry
	for _, uid := range userIDs {
		solvedMap := userSolves[uid]
		var lastSolveAt time.Time
		results := make([]challengeResult, 0, len(chalIDs))

		for _, chalID := range chalIDs {
			cr := challengeResult{ChallengeID: chalID}
			if solvedAt, ok := solvedMap[chalID]; ok {
				cr.Solved = true
				cr.SolvedAt = solvedAt
				if br, hasBr := bloodRank[uid+":"+chalID]; hasBr {
					cr.BloodRank = br
				}
				if solvedAt.After(lastSolveAt) {
					lastSolveAt = solvedAt
				}
			}
			results = append(results, cr)
		}

		board = append(board, entry{
			UserID:        uid,
			TotalScore:    totalScores[uid],
			LastSolveTime: lastSolveAt,
			Challenges:    results,
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
	pluginutil.WriteJSON(w, http.StatusOK, map[string]any{"leaderboard": board})
}
