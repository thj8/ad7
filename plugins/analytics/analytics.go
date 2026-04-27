// Package analytics 实现比赛分析插件。
// 提供四个维度的分析接口：总览、分类统计、用户统计、题目统计。
// 所有接口需要认证，数据限定在比赛范围内。
package analytics

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/pluginutil"
)

// Plugin 是分析插件，持有数据库连接。
type Plugin struct{ db *sql.DB }

// New 创建分析插件实例。
func New() *Plugin { return &Plugin{} }

// Name 返回插件名称
func (p *Plugin) Name() string {
	return plugin.NameAnalytics
}

// Register 注册分析相关的路由。
// 路由（均需认证）：
//   - GET /api/v1/competitions/{id}/analytics/overview（总览）
//   - GET /api/v1/competitions/{id}/analytics/categories（分类统计）
//   - GET /api/v1/competitions/{id}/analytics/users（用户统计）
//   - GET /api/v1/competitions/{id}/analytics/challenges（题目统计）
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/overview", p.overview)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/categories", p.byCategory)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/users", p.userStats)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/challenges", p.challengeStats)
}

// overviewResponse 是比赛总览分析的响应结构。
type overviewResponse struct {
	TotalUsers         int     `json:"total_users,omitempty"`     // 参赛总人数（个人模式）
	TotalTeams         int     `json:"total_teams,omitempty"`     // 参赛总队数（队伍模式）
	TotalChallenges    int     `json:"total_challenges"`          // 题目总数
	TotalSubmissions   int     `json:"total_submissions"`         // 总提交数
	CorrectSubmissions int     `json:"correct_submissions"`       // 正确提交数
	AverageSolves      float64 `json:"average_solves"`            // 人均/队均解题数
	AverageSolveTime   string  `json:"average_solve_time_seconds"`// 平均解题时间（秒）
	CompletionRate     float64 `json:"completion_rate"`           // 完成率（%）
}

// categoryResponse 是分类统计的响应结构。
type categoryResponse struct {
	Categories []categoryStats `json:"categories"`
}

// categoryStats 是单个分类的统计数据。
type categoryStats struct {
	Category          string  `json:"category"`                   // 分类名称
	TotalChallenges   int     `json:"total_challenges"`           // 该分类题目数
	TotalSolves       int     `json:"total_solves"`               // 该分类总解题数
	UniqueUsersSolved int     `json:"unique_users_solved,omitempty"`  // 该分类独立解题用户数（个人模式）
	UniqueTeamsSolved int     `json:"unique_teams_solved,omitempty"`  // 该分类独立解题队伍数（队伍模式）
	AverageSolves     float64 `json:"average_solves_per_user,omitempty"`  // 人均解题数（个人模式）
	AverageSolvesPerTeam float64 `json:"average_solves_per_team,omitempty"` // 队均解题数（队伍模式）
	SuccessRate       float64 `json:"success_rate"`               // 成功率（%）
}

// userStatsResponse 是用户统计的响应结构。
type userStatsResponse struct {
	Users []userStats `json:"users,omitempty"`
	Teams []teamStats `json:"teams,omitempty"`
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

// teamStats 是单个队伍的统计数据。
type teamStats struct {
	TeamID         string  `json:"team_id"`          // 队伍 ID
	TotalSolves    int     `json:"total_solves"`     // 正确解题数
	TotalScore     int     `json:"total_score"`      // 总得分
	TotalAttempts  int     `json:"total_attempts"`   // 总提交次数
	SuccessRate    float64 `json:"success_rate"`     // 成功率（%）
	FirstSolveTime string  `json:"first_solve_time"` // 首次解题时间（RFC3339）
	LastSolveTime  string  `json:"last_solve_time"`  // 最后解题时间（RFC3339）
}

// challengeStatsResponse 是题目统计的响应结构。
type challengeStatsResponse struct {
	Challenges []challengeStats `json:"challenges"`
}

// challengeStats 是单个题目的统计数据。
type challengeStats struct {
	ChallengeID       string  `json:"challenge_id"`               // 题目 ID
	Title             string  `json:"title"`                      // 题目标题
	Category          string  `json:"category"`                   // 题目分类
	Score             int     `json:"score"`                      // 题目分值
	TotalSolves       int     `json:"total_solves"`               // 正确解题数
	TotalAttempts     int     `json:"total_attempts"`             // 总提交次数
	SuccessRate       float64 `json:"success_rate"`               // 成功率（%）
	UniqueUsersSolved int     `json:"unique_users_solved,omitempty"` // 独立解题用户数（个人模式）
	UniqueTeamsSolved int     `json:"unique_teams_solved,omitempty"` // 独立解题队伍数（队伍模式）
	FirstSolveTime    string  `json:"first_solve_time"`           // 首次解题时间（RFC3339）
	AverageSolveTime  string  `json:"average_solve_time_seconds"` // 平均解题时间（秒）
}

// overview 处理比赛总览分析请求。
func (p *Plugin) overview(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	ctx := r.Context()
	var resp overviewResponse

	// 获取比赛模式
	var mode string
	err := p.db.QueryRowContext(ctx, `
		SELECT mode FROM competitions WHERE res_id = ? AND is_deleted = 0
	`, compID).Scan(&mode)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	// 查询比赛中的题目总数
	totalChallenges, err := pluginutil.GetCompChallengeCount(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp.TotalChallenges = totalChallenges

	var totalEntities int
	if mode == "team" {
		// 查询有提交记录的队伍数
		totalTeams, err := pluginutil.GetCompDistinctTeams(ctx, p.db, compID)
		if err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}
		resp.TotalTeams = totalTeams
		totalEntities = totalTeams
	} else {
		// 查询有提交记录的用户数
		totalUsers, err := pluginutil.GetCompDistinctUsers(ctx, p.db, compID)
		if err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}
		resp.TotalUsers = totalUsers
		totalEntities = totalUsers
	}

	// 查询总提交数和正确提交数
	totalSubs, correctSubs, err := pluginutil.GetCompSubmitStats(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp.TotalSubmissions = totalSubs
	resp.CorrectSubmissions = correctSubs

	// 计算人均/队均解题数和完成率
	if totalEntities > 0 {
		// 注意：这里correctSubs是总正确提交数，不管模式，我们需要根据模式获取正确解题数
		var correctSolves int
		if mode == "team" {
			teamSolves, err := pluginutil.GetTeamCorrectSubmissions(ctx, p.db, compID)
			if err == nil {
				correctSolves = len(teamSolves)
			}
		} else {
			userSolves, err := pluginutil.GetCorrectSubmissions(ctx, p.db, compID)
			if err == nil {
				correctSolves = len(userSolves)
			}
		}
		if correctSolves == 0 {
			correctSolves = correctSubs // 回退到总正确提交数
		}
		resp.AverageSolves = float64(correctSolves) / float64(totalEntities)
		if resp.TotalChallenges > 0 {
			resp.CompletionRate = (resp.AverageSolves / float64(resp.TotalChallenges)) * 100
		}
	}

	// 计算平均解题时间（从比赛开始到首次正确提交的平均秒数）
	avgSolveTime, err := pluginutil.GetAverageSolveTimeFromStart(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp.AverageSolveTime = strconv.FormatFloat(avgSolveTime, 'f', 2, 64)

	pluginutil.WriteJSON(w, http.StatusOK, resp)
}

// byCategory 处理按分类统计的请求。
func (p *Plugin) byCategory(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	ctx := r.Context()

	// 获取比赛模式
	var mode string
	err := p.db.QueryRowContext(ctx, `
		SELECT mode FROM competitions WHERE res_id = ? AND is_deleted = 0
	`, compID).Scan(&mode)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	var totalEntities int
	if mode == "team" {
		totalEntities, err = pluginutil.GetCompDistinctTeams(ctx, p.db, compID)
	} else {
		totalEntities, err = pluginutil.GetCompDistinctUsers(ctx, p.db, compID)
	}
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	// 使用共享查询函数获取各分类统计
	catStats, err := pluginutil.GetCategoryStats(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	var categories []categoryStats
	for _, cat := range catStats {
		cs := categoryStats{
			Category:        cat.Category,
			TotalChallenges: cat.TotalChallenges,
			TotalSolves:     cat.TotalSolves,
		}
		if mode == "team" {
			// 获取该分类的独立解题队伍数
			var uniqueTeamsSolved int
			p.db.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT s.team_id)
				FROM submissions s
				INNER JOIN competition_challenges cc ON s.challenge_id = cc.challenge_id AND s.competition_id = cc.competition_id
				INNER JOIN challenges c ON c.res_id = cc.challenge_id
				WHERE cc.competition_id = ? AND cc.is_deleted = 0 AND c.is_deleted = 0
				AND s.is_correct = 1 AND s.is_deleted = 0 AND s.team_id != ''
				AND c.category = ?
			`, compID, cat.Category).Scan(&uniqueTeamsSolved)
			cs.UniqueTeamsSolved = uniqueTeamsSolved
			if totalEntities > 0 {
				cs.AverageSolvesPerTeam = float64(cat.TotalSolves) / float64(totalEntities)
			}
		} else {
			cs.UniqueUsersSolved = cat.UniqueUsersSolved
			if totalEntities > 0 {
				cs.AverageSolves = float64(cat.TotalSolves) / float64(totalEntities)
			}
		}
		if cat.TotalAttempts > 0 {
			cs.SuccessRate = (float64(cat.TotalSolves) / float64(cat.TotalAttempts)) * 100
		}
		categories = append(categories, cs)
	}

	if categories == nil {
		categories = []categoryStats{}
	}

	pluginutil.WriteJSON(w, http.StatusOK, categoryResponse{Categories: categories})
}

// userStats 处理用户/队伍统计请求。
func (p *Plugin) userStats(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	ctx := r.Context()

	// 获取比赛模式
	var mode string
	err := p.db.QueryRowContext(ctx, `
		SELECT mode FROM competitions WHERE res_id = ? AND is_deleted = 0
	`, compID).Scan(&mode)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	resp := userStatsResponse{}
	if mode == "team" {
		// 使用共享查询函数获取队伍完整统计
		teamFullStats, err := pluginutil.GetTeamFullStats(ctx, p.db, compID)
		if err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}

		var teams []teamStats
		for _, t := range teamFullStats {
			ts := teamStats{
				TeamID:        t.TeamID,
				TotalSolves:   t.TotalSolves,
				TotalScore:    t.TotalScore,
				TotalAttempts: t.TotalAttempts,
			}
			if t.TotalAttempts > 0 {
				ts.SuccessRate = (float64(t.TotalSolves) / float64(t.TotalAttempts)) * 100
			}
			if t.FirstSolveAt != nil {
				ts.FirstSolveTime = t.FirstSolveAt.Format(time.RFC3339)
			}
			if t.LastSolveAt != nil {
				ts.LastSolveTime = t.LastSolveAt.Format(time.RFC3339)
			}
			teams = append(teams, ts)
		}

		if teams == nil {
			teams = []teamStats{}
		}
		resp.Teams = teams
	} else {
		// 使用共享查询函数获取用户完整统计
		userFullStats, err := pluginutil.GetUserFullStats(ctx, p.db, compID)
		if err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}

		var users []userStats
		for _, u := range userFullStats {
			us := userStats{
				UserID:        u.UserID,
				TotalSolves:   u.TotalSolves,
				TotalScore:    u.TotalScore,
				TotalAttempts: u.TotalAttempts,
			}
			if u.TotalAttempts > 0 {
				us.SuccessRate = (float64(u.TotalSolves) / float64(u.TotalAttempts)) * 100
			}
			if u.FirstSolveAt != nil {
				us.FirstSolveTime = u.FirstSolveAt.Format(time.RFC3339)
			}
			if u.LastSolveAt != nil {
				us.LastSolveTime = u.LastSolveAt.Format(time.RFC3339)
			}
			users = append(users, us)
		}

		if users == nil {
			users = []userStats{}
		}
		resp.Users = users
	}

	pluginutil.WriteJSON(w, http.StatusOK, resp)
}

// challengeStats 处理题目统计请求。
func (p *Plugin) challengeStats(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	ctx := r.Context()

	// 获取比赛模式
	var mode string
	err := p.db.QueryRowContext(ctx, `
		SELECT mode FROM competitions WHERE res_id = ? AND is_deleted = 0
	`, compID).Scan(&mode)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	// 查询比赛中所有题目（使用共享查询函数）
	challenges, err := pluginutil.GetCompChallenges(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	// 获取所有题目的完整统计
	challengeFullStats, err := pluginutil.GetChallengeFullStats(ctx, p.db, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	// 构建 challengeID -> stat 的映射
	statMap := make(map[string]pluginutil.ChallengeFullStat)
	for _, stat := range challengeFullStats {
		statMap[stat.ChallengeID] = stat
	}

	var result []challengeStats
	for _, ch := range challenges {
		cs := challengeStats{
			ChallengeID: ch.ResID,
			Title:       ch.Title,
			Category:    ch.Category,
			Score:       ch.Score,
		}

		// 从统计映射中获取数据
		if stat, ok := statMap[ch.ResID]; ok {
			cs.TotalSolves = stat.TotalSolves
			cs.TotalAttempts = stat.TotalAttempts
			if stat.FirstSolveAt != nil {
				cs.FirstSolveTime = stat.FirstSolveAt.Format(time.RFC3339)
			}
			cs.AverageSolveTime = strconv.FormatFloat(stat.AvgSolveTimeSec, 'f', 2, 64)
		}

		// 根据模式获取独立解题数
		if mode == "team" {
			var uniqueTeamsSolved int
			p.db.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT s.team_id)
				FROM submissions s
				WHERE s.competition_id = ? AND s.challenge_id = ? AND s.is_correct = 1 AND s.is_deleted = 0 AND s.team_id != ''
			`, compID, ch.ResID).Scan(&uniqueTeamsSolved)
			cs.UniqueTeamsSolved = uniqueTeamsSolved
		} else {
			if stat, ok := statMap[ch.ResID]; ok {
				cs.UniqueUsersSolved = stat.UniqueUsersSolved
			}
		}

		if cs.TotalAttempts > 0 {
			cs.SuccessRate = (float64(cs.TotalSolves) / float64(cs.TotalAttempts)) * 100
		}

		result = append(result, cs)
	}

	if result == nil {
		result = []challengeStats{}
	}

	pluginutil.WriteJSON(w, http.StatusOK, challengeStatsResponse{Challenges: result})
}
