// Package analytics 实现比赛分析插件。
// 提供四个维度的分析接口：总览、分类统计、用户统计、题目统计。
// 所有接口需要认证，数据限定在比赛范围内。
package analytics

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/pluginutil"
)

// Plugin 是分析插件，持有数据库连接。
type Plugin struct{ db *sql.DB }

// New 创建分析插件实例。
func New() *Plugin { return &Plugin{} }

// Register 注册分析相关的路由。
// 路由（均需认证）：
//   - GET /api/v1/competitions/{id}/analytics/overview（总览）
//   - GET /api/v1/competitions/{id}/analytics/categories（分类统计）
//   - GET /api/v1/competitions/{id}/analytics/users（用户统计）
//   - GET /api/v1/competitions/{id}/analytics/challenges（题目统计）
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/overview", p.overview)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/categories", p.byCategory)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/users", p.userStats)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/challenges", p.challengeStats)
}

// overviewResponse 是比赛总览分析的响应结构。
type overviewResponse struct {
	TotalUsers         int     `json:"total_users"`          // 参赛总人数
	TotalChallenges    int     `json:"total_challenges"`     // 题目总数
	TotalSubmissions   int     `json:"total_submissions"`    // 总提交数
	CorrectSubmissions int     `json:"correct_submissions"`  // 正确提交数
	AverageSolves      float64 `json:"average_solves"`       // 人均解题数
	AverageSolveTime   string  `json:"average_solve_time_seconds"` // 平均解题时间（秒）
	CompletionRate     float64 `json:"completion_rate"`      // 完成率（%）
}

// categoryStats 是单个分类的统计数据。
type categoryStats struct {
	Category          string  `json:"category"`                  // 分类名称
	TotalChallenges   int     `json:"total_challenges"`          // 该分类题目数
	TotalSolves       int     `json:"total_solves"`              // 该分类总解题数
	UniqueUsersSolved int     `json:"unique_users_solved"`       // 该分类独立解题用户数
	TotalAttempts     int     `json:"-"`                         // 该分类总提交数（仅用于计算成功率）
	AverageSolves     float64 `json:"average_solves_per_user"`   // 人均解题数
	SuccessRate       float64 `json:"success_rate"`              // 成功率（%）
}

// categoryResponse 是分类统计的响应结构。
type categoryResponse struct {
	Categories []categoryStats `json:"categories"`
}

// overview 处理比赛总览分析请求。
func (p *Plugin) overview(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	slog.Info("analytics overview", "competition_id", compID)
	ctx := r.Context()
	var resp overviewResponse

	// 查询比赛中的题目总数
	totalChallenges, err := pluginutil.GetCompChallengeCount(ctx, p.db, compID)
	if err != nil {
		slog.Error("analytics overview query failed", "error", err, "competition_id", compID)
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp.TotalChallenges = totalChallenges

	// 查询有提交记录的用户数
	totalUsers, err := pluginutil.GetCompDistinctUsers(ctx, p.db, compID)
	if err != nil {
		slog.Error("analytics overview query failed", "error", err, "competition_id", compID)
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp.TotalUsers = totalUsers

	// 查询总提交数和正确提交数
	totalSubs, correctSubs, err := pluginutil.GetCompSubmitStats(ctx, p.db, compID)
	if err != nil {
		slog.Error("analytics overview query failed", "error", err, "competition_id", compID)
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp.TotalSubmissions = totalSubs
	resp.CorrectSubmissions = correctSubs

	// 计算人均解题数和完成率
	if resp.TotalUsers > 0 {
		resp.AverageSolves = float64(correctSubs) / float64(resp.TotalUsers)
		if resp.TotalChallenges > 0 {
			resp.CompletionRate = (resp.AverageSolves / float64(resp.TotalChallenges)) * 100
		}
	}

	// 计算平均解题时间（从比赛开始到首次正确提交的平均秒数）
	var avgSolveTimeSec sql.NullFloat64
	err = p.db.QueryRowContext(ctx, `
		SELECT AVG(TIMESTAMPDIFF(SECOND, c.start_time, s.created_at))
		FROM submissions s
		JOIN competitions c ON c.res_id = s.competition_id
		WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0 AND c.is_deleted = 0
	`, compID).Scan(&avgSolveTimeSec)
	if err != nil {
		slog.Error("analytics overview query failed", "error", err, "competition_id", compID)
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	if avgSolveTimeSec.Valid {
		resp.AverageSolveTime = strconv.FormatFloat(avgSolveTimeSec.Float64, 'f', 2, 64)
	} else {
		resp.AverageSolveTime = "0"
	}

	pluginutil.WriteJSON(w, http.StatusOK, resp)
}

// byCategory 处理按分类统计的请求。
func (p *Plugin) byCategory(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	slog.Info("analytics byCategory", "competition_id", compID)
	ctx := r.Context()

	// 查询比赛总用户数
	totalUsers, err := pluginutil.GetCompDistinctUsers(ctx, p.db, compID)
	if err != nil {
		slog.Error("analytics byCategory query failed", "error", err, "competition_id", compID)
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	// 单次聚合查询获取各分类的题目数、正确解题数、独立用户数和总提交数
	rows, err := p.db.QueryContext(ctx, `
		SELECT
			c.category,
			COUNT(DISTINCT cc.challenge_id) as total_challenges,
			COALESCE(SUM(CASE WHEN s.is_correct = 1 THEN 1 ELSE 0 END), 0) as total_solves,
			COUNT(DISTINCT CASE WHEN s.is_correct = 1 THEN s.user_id ELSE NULL END) as unique_users_solved,
			COUNT(s.id) as total_attempts
		FROM competition_challenges cc
		JOIN challenges c ON c.res_id = cc.challenge_id AND c.is_deleted = 0
		LEFT JOIN submissions s ON s.challenge_id = cc.challenge_id
			AND s.competition_id = cc.competition_id AND s.is_deleted = 0
		WHERE cc.competition_id = ? AND cc.is_deleted = 0
		GROUP BY c.category
	`, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	defer rows.Close()

	var categories []categoryStats
	for rows.Next() {
		var cat categoryStats
		if err := rows.Scan(&cat.Category, &cat.TotalChallenges, &cat.TotalSolves, &cat.UniqueUsersSolved, &cat.TotalAttempts); err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}

		if totalUsers > 0 {
			cat.AverageSolves = float64(cat.TotalSolves) / float64(totalUsers)
		}
		if cat.TotalAttempts > 0 {
			cat.SuccessRate = (float64(cat.TotalSolves) / float64(cat.TotalAttempts)) * 100
		}

		categories = append(categories, cat)
	}

	if categories == nil {
		categories = []categoryStats{}
	}

	pluginutil.WriteJSON(w, http.StatusOK, categoryResponse{Categories: categories})
}

// userStats 是单个用户的统计数据。
type userStats struct {
	UserID         string  `json:"user_id"`          // 用户 ID
	TotalSolves    int     `json:"total_solves"`     // 正确解题数
	TotalScore     int     `json:"total_score"`      // 总得分
	TotalAttempts  int     `json:"total_attempts"`   // 总提交次数
	SuccessRate    float64 `json:"success_rate"`     // 成功率（%）
	FirstSolveTime string  `json:"first_solve_time"` // 首次解题时间（RFC3339）
	LastSolveTime  string  `json:"last_solve_time"`  // 最后解题时间（RFC3339）
}

// userStatsResponse 是用户统计的响应结构。
type userStatsResponse struct {
	Users []userStats `json:"users"`
}

// userStats 处理用户统计请求。
// 复杂聚合查询，不适合提取到共享函数，保持内联。
func (p *Plugin) userStats(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	ctx := r.Context()

	rows, err := p.db.QueryContext(ctx, `
		SELECT
			s.user_id,
			SUM(CASE WHEN s.is_correct = 1 THEN 1 ELSE 0 END) as total_solves,
			SUM(CASE WHEN s.is_correct = 1 THEN c.score ELSE 0 END) as total_score,
			COUNT(*) as total_attempts,
			MIN(CASE WHEN s.is_correct = 1 THEN s.created_at ELSE NULL END) as first_solve,
			MAX(CASE WHEN s.is_correct = 1 THEN s.created_at ELSE NULL END) as last_solve
		FROM submissions s
		LEFT JOIN challenges c ON c.res_id = s.challenge_id AND c.is_deleted = 0
		WHERE s.competition_id = ? AND s.is_deleted = 0
		GROUP BY s.user_id
		ORDER BY total_score DESC, first_solve ASC
		`, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	defer rows.Close()

	var users []userStats
	for rows.Next() {
		var u userStats
		var firstSolve, lastSolve sql.NullTime
		if err := rows.Scan(
			&u.UserID,
			&u.TotalSolves,
			&u.TotalScore,
			&u.TotalAttempts,
			&firstSolve,
			&lastSolve,
		); err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}

		if u.TotalAttempts > 0 {
			u.SuccessRate = (float64(u.TotalSolves) / float64(u.TotalAttempts)) * 100
		}

		if firstSolve.Valid {
			u.FirstSolveTime = firstSolve.Time.Format(time.RFC3339)
		}
		if lastSolve.Valid {
			u.LastSolveTime = lastSolve.Time.Format(time.RFC3339)
		}

		users = append(users, u)
	}

	if users == nil {
		users = []userStats{}
	}

	pluginutil.WriteJSON(w, http.StatusOK, userStatsResponse{Users: users})
}

// challengeStats 是单个题目的统计数据。
type challengeStats struct {
	ChallengeID       string  `json:"challenge_id"`            // 题目 ID
	Title             string  `json:"title"`                   // 题目标题
	Category          string  `json:"category"`                // 题目分类
	Score             int     `json:"score"`                   // 题目分值
	TotalSolves       int     `json:"total_solves"`            // 正确解题数
	TotalAttempts     int     `json:"total_attempts"`          // 总提交次数
	SuccessRate       float64 `json:"success_rate"`            // 成功率（%）
	UniqueUsersSolved int     `json:"unique_users_solved"`     // 独立解题用户数
	FirstSolveTime    string  `json:"first_solve_time"`        // 首次解题时间（RFC3339）
	AverageSolveTime  string  `json:"average_solve_time_seconds"` // 平均解题时间（秒）
}

// challengeStatsResponse 是题目统计的响应结构。
type challengeStatsResponse struct {
	Challenges []challengeStats `json:"challenges"`
}

// challengeStats 处理题目统计请求。
func (p *Plugin) challengeStats(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	ctx := r.Context()

	// 查询比赛中所有题目（使用共享查询函数）
	challenges, err := pluginutil.GetCompChallenges(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	var result []challengeStats
	for _, ch := range challenges {
		cs := challengeStats{
			ChallengeID: ch.ResID,
			Title:       ch.Title,
			Category:    ch.Category,
			Score:       ch.Score,
		}

		// 查询该题目的提交统计
		var firstSolve sql.NullTime
		err = p.db.QueryRowContext(ctx, `
			SELECT
				SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END),
				COUNT(*),
				COUNT(DISTINCT CASE WHEN is_correct = 1 THEN user_id ELSE NULL END),
				MIN(CASE WHEN is_correct = 1 THEN created_at ELSE NULL END)
			FROM submissions
			WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 0
			`, compID, cs.ChallengeID).Scan(
			&cs.TotalSolves,
			&cs.TotalAttempts,
			&cs.UniqueUsersSolved,
			&firstSolve,
		)
		if err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}

		if cs.TotalAttempts > 0 {
			cs.SuccessRate = (float64(cs.TotalSolves) / float64(cs.TotalAttempts)) * 100
		}

		if firstSolve.Valid {
			cs.FirstSolveTime = firstSolve.Time.Format(time.RFC3339)
		}

		// 计算平均解题时间（每个用户从首次提交到正确提交的平均秒数）
		var avgSolveTime sql.NullFloat64
		err = p.db.QueryRowContext(ctx, `
			SELECT AVG(TIMESTAMPDIFF(SECOND, first_submit, correct_submit))
			FROM (
				SELECT
					user_id,
					MIN(created_at) as first_submit,
					MIN(CASE WHEN is_correct = 1 THEN created_at ELSE NULL END) as correct_submit
				FROM submissions
				WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 0
				GROUP BY user_id
				HAVING correct_submit IS NOT NULL
			) user_times
			`, compID, cs.ChallengeID).Scan(&avgSolveTime)
		if err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}
		if avgSolveTime.Valid {
			cs.AverageSolveTime = strconv.FormatFloat(avgSolveTime.Float64, 'f', 2, 64)
		} else {
			cs.AverageSolveTime = "0"
		}

		result = append(result, cs)
	}

	if result == nil {
		result = []challengeStats{}
	}

	pluginutil.WriteJSON(w, http.StatusOK, challengeStatsResponse{Challenges: result})
}
