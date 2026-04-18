// Package leaderboard 实现比赛排行榜插件。
// 按比赛维度统计用户的总得分和最后解题时间，
// 排行规则：总分降序，同分按最后正确提交时间升序（越早越好）。
package leaderboard

import (
	"database/sql"
	"encoding/json"
	"net/http"
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

// entry 是排行榜中的一条记录，表示一个用户的排名信息。
type entry struct {
	Rank          int       `json:"rank"`            // 排名（从 1 开始）
	UserID        string    `json:"user_id"`         // 用户 ID
	TotalScore    int       `json:"total_score"`     // 总得分
	LastSolveTime time.Time `json:"last_solve_time"` // 最后一次正确提交的时间（用于同分排序）
}

// listByComp 处理获取比赛排行榜的请求。
// 查询指定比赛中所有正确提交，按用户分组计算总分和最后解题时间，
// 按总分降序、最后解题时间升序排列。
func (p *Plugin) listByComp(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	// 查询排行榜数据：按用户分组，计算总分和最后解题时间
	rows, err := p.db.QueryContext(r.Context(), `
		SELECT s.user_id, SUM(c.score), MAX(s.created_at)
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0
		GROUP BY s.user_id
		ORDER BY SUM(c.score) DESC, MAX(s.created_at) ASC`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var board []entry
	rank := 1
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.UserID, &e.TotalScore, &e.LastSolveTime); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		e.Rank = rank
		rank++
		board = append(board, e)
	}
	// 确保空排行榜返回 [] 而非 null
	if board == nil {
		board = []entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"leaderboard": board})
}
