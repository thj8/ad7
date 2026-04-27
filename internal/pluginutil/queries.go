// Package pluginutil 提供插件共享的数据库查询函数。
// 封装对主程序表（challenges、submissions、competition_challenges）的常用查询，
// 插件私有表（topthree_records、hints、notifications）不在此处访问。
package pluginutil

import (
	"context"
	"database/sql"
	"time"
)

// DBTX 是数据库查询接口，兼容 *sql.DB 和 *sql.Tx。
// 插件传入 *sql.DB 即可使用所有共享查询函数。
type DBTX interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ChallengeInfo 题目摘要信息（不含 Flag 和 description）。
type ChallengeInfo struct {
	ResID    string
	Title    string
	Category string
	Score    int
}

// FirstSolve 用户在某道题目的最早正确提交记录。
type FirstSolve struct {
	UserID      string
	ChallengeID string
	SolvedAt    time.Time
}

// TeamFirstSolve 队伍在某道题目的最早正确提交记录。
type TeamFirstSolve struct {
	TeamID      string
	ChallengeID string
	SolvedAt    time.Time
}

// GetCompChallenges 获取比赛中所有已启用且未删除的题目摘要信息。
// 按题目 res_id 排序。调用方只需 ID 列表时，从返回值中提取 .ResID 即可。
func GetCompChallenges(ctx context.Context, db DBTX, compID string) ([]ChallengeInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.res_id, c.title, c.category, c.score
		FROM competition_challenges cc
		JOIN challenges c ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ?
		  AND cc.is_deleted = 0
		  AND c.is_enabled = 1
		  AND c.is_deleted = 0
		ORDER BY c.res_id`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []ChallengeInfo
	for rows.Next() {
		var ci ChallengeInfo
		if err := rows.Scan(&ci.ResID, &ci.Title, &ci.Category, &ci.Score); err != nil {
			return nil, err
		}
		challenges = append(challenges, ci)
	}
	return challenges, rows.Err()
}

// GetCorrectSubmissions 获取比赛中每用户每题的正确提交。
// 每个用户对每道题最多一条记录（通过 GROUP BY 去重）。
func GetCorrectSubmissions(ctx context.Context, db DBTX, compID string) ([]FirstSolve, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.user_id, s.challenge_id, MIN(s.created_at)
		FROM submissions s
		WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0
		GROUP BY s.user_id, s.challenge_id`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var solves []FirstSolve
	for rows.Next() {
		var fs FirstSolve
		if err := rows.Scan(&fs.UserID, &fs.ChallengeID, &fs.SolvedAt); err != nil {
			return nil, err
		}
		solves = append(solves, fs)
	}
	return solves, rows.Err()
}

// GetTeamCorrectSubmissions 获取比赛中每队每题的正确提交。
// 每个队伍对每道题最多一条记录（通过 GROUP BY 去重）。
func GetTeamCorrectSubmissions(ctx context.Context, db DBTX, compID string) ([]TeamFirstSolve, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.team_id, s.challenge_id, MIN(s.created_at)
		FROM submissions s
		WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0 AND s.team_id != ''
		GROUP BY s.team_id, s.challenge_id`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var solves []TeamFirstSolve
	for rows.Next() {
		var fs TeamFirstSolve
		if err := rows.Scan(&fs.TeamID, &fs.ChallengeID, &fs.SolvedAt); err != nil {
			return nil, err
		}
		solves = append(solves, fs)
	}
	return solves, rows.Err()
}

// GetUserScores 获取比赛中每用户的总得分。
// 通过 JOIN challenges 表计算正确提交对应的分数之和。
func GetUserScores(ctx context.Context, db DBTX, compID string) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.user_id, SUM(c.score)
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0
		GROUP BY s.user_id`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scores := make(map[string]int)
	for rows.Next() {
		var uid string
		var score int
		if err := rows.Scan(&uid, &score); err != nil {
			return nil, err
		}
		scores[uid] = score
	}
	return scores, rows.Err()
}

// GetTeamScores 获取比赛中每队的总得分。
// 通过 JOIN challenges 表计算正确提交对应的分数之和。
func GetTeamScores(ctx context.Context, db DBTX, compID string) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.team_id, SUM(c.score)
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0 AND s.team_id != ''
		GROUP BY s.team_id`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scores := make(map[string]int)
	for rows.Next() {
		var tid string
		var score int
		if err := rows.Scan(&tid, &score); err != nil {
			return nil, err
		}
		scores[tid] = score
	}
	return scores, rows.Err()
}

// GetCompSubmitStats 获取比赛的总提交数和正确提交数。
// 返回 (总数, 正确数, 错误)。
func GetCompSubmitStats(ctx context.Context, db DBTX, compID string) (int, int, error) {
	var total, correct int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END), 0)
		FROM submissions WHERE competition_id = ? AND is_deleted = 0
	`, compID).Scan(&total, &correct)
	return total, correct, err
}

// GetCompDistinctUsers 获取比赛中有提交记录的独立用户数。
func GetCompDistinctUsers(ctx context.Context, db DBTX, compID string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM submissions WHERE competition_id = ? AND is_deleted = 0
	`, compID).Scan(&count)
	return count, err
}

// GetCompChallengeCount 获取比赛中的题目总数（通过 competition_challenges 关联表统计）。
func GetCompChallengeCount(ctx context.Context, db DBTX, compID string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM competition_challenges WHERE competition_id = ? AND is_deleted = 0
	`, compID).Scan(&count)
	return count, err
}

// ScoreDistribution 分数分布统计
type ScoreDistribution struct {
	Zero      int // 0分
	OneHundred    int // 1-100分
	FiveHundred   int // 101-500分
	OneThousand   int // 501-1000分
	OverThousand  int // 1000分以上
}

// GetScoreDistribution 获取比赛得分分布统计
// 按用户总分分组统计各区间人数
func GetScoreDistribution(ctx context.Context, db DBTX, compID string) (*ScoreDistribution, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			CASE
				WHEN user_score = 0 THEN 'zero'
				WHEN user_score <= 100 THEN 'one_hundred'
				WHEN user_score <= 500 THEN 'five_hundred'
				WHEN user_score <= 1000 THEN 'one_thousand'
				ELSE 'over_thousand'
			END as score_range,
			COUNT(*) as count
		FROM (
			SELECT s.user_id, SUM(c.score) as user_score
			FROM submissions s
			JOIN challenges c ON c.res_id = s.challenge_id
			WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0
			GROUP BY s.user_id
		) t
		GROUP BY score_range
	`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dist := &ScoreDistribution{}
	for rows.Next() {
		var rangeName string
		var count int
		if err := rows.Scan(&rangeName, &count); err != nil {
			return nil, err
		}
		switch rangeName {
		case "zero":
			dist.Zero = count
		case "one_hundred":
			dist.OneHundred = count
		case "five_hundred":
			dist.FiveHundred = count
		case "one_thousand":
			dist.OneThousand = count
		case "over_thousand":
			dist.OverThousand = count
		}
	}
	return dist, rows.Err()
}

// ChallengeStat 单题统计信息
type ChallengeStat struct {
	ChallengeID   string
	Title         string
	Category      string
	Score         int
	FirstBloodAt  *time.Time // 一血时间，可能为nil
	TotalSolves   int        // 总解题人数
}

// GetChallengeStats 获取比赛各题目的统计信息
// 包括一血时间和总解题人数
func GetChallengeStats(ctx context.Context, db DBTX, compID string) ([]ChallengeStat, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			c.res_id,
			c.title,
			c.category,
			c.score,
			MIN(CASE WHEN s.is_correct = 1 THEN s.created_at END) as first_blood_at,
			COUNT(DISTINCT CASE WHEN s.is_correct = 1 THEN s.user_id END) as total_solves
		FROM competition_challenges cc
		JOIN challenges c ON c.res_id = cc.challenge_id
		LEFT JOIN submissions s ON s.challenge_id = c.res_id AND s.competition_id = cc.competition_id AND s.is_deleted = 0
		WHERE cc.competition_id = ? AND cc.is_deleted = 0 AND c.is_deleted = 0
		GROUP BY c.res_id, c.title, c.category, c.score
		ORDER BY c.res_id
	`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ChallengeStat
	for rows.Next() {
		var s ChallengeStat
		var firstBloodAt sql.NullTime
		if err := rows.Scan(&s.ChallengeID, &s.Title, &s.Category, &s.Score, &firstBloodAt, &s.TotalSolves); err != nil {
			return nil, err
		}
		if firstBloodAt.Valid {
			s.FirstBloodAt = &firstBloodAt.Time
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// UserSolveStat 用户解题统计
type UserSolveStat struct {
	UserID      string
	TotalScore  int
	SolveCount  int
	LastSolveAt *time.Time
}

// GetUserSolveStats 获取比赛中所有用户的解题统计
// 按总分降序、解题数降序排列
func GetUserSolveStats(ctx context.Context, db DBTX, compID string) ([]UserSolveStat, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			s.user_id,
			SUM(c.score) as total_score,
			COUNT(DISTINCT s.challenge_id) as solve_count,
			MAX(s.created_at) as last_solve_at
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0
		GROUP BY s.user_id
		ORDER BY total_score DESC, solve_count DESC, last_solve_at ASC
	`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []UserSolveStat
	for rows.Next() {
		var s UserSolveStat
		var lastSolveAt sql.NullTime
		if err := rows.Scan(&s.UserID, &s.TotalScore, &s.SolveCount, &lastSolveAt); err != nil {
			return nil, err
		}
		if lastSolveAt.Valid {
			s.LastSolveAt = &lastSolveAt.Time
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// TeamSolveStat 队伍解题统计
type TeamSolveStat struct {
	TeamID      string
	TotalScore  int
	SolveCount  int
	LastSolveAt *time.Time
}

// GetTeamSolveStats 获取比赛中所有队伍的解题统计
// 按总分降序、解题数降序排列
func GetTeamSolveStats(ctx context.Context, db DBTX, compID string) ([]TeamSolveStat, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			s.team_id,
			SUM(c.score) as total_score,
			COUNT(DISTINCT s.challenge_id) as solve_count,
			MAX(s.created_at) as last_solve_at
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0 AND s.team_id != ''
		GROUP BY s.team_id
		ORDER BY total_score DESC, solve_count DESC, last_solve_at ASC
	`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TeamSolveStat
	for rows.Next() {
		var s TeamSolveStat
		var lastSolveAt sql.NullTime
		if err := rows.Scan(&s.TeamID, &s.TotalScore, &s.SolveCount, &lastSolveAt); err != nil {
			return nil, err
		}
		if lastSolveAt.Valid {
			s.LastSolveAt = &lastSolveAt.Time
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// GetCompetitionMode 获取比赛的模式
func GetCompetitionMode(ctx context.Context, db DBTX, compID string) (string, error) {
	var mode string
	err := db.QueryRowContext(ctx, `
		SELECT mode FROM competitions WHERE res_id = ? AND is_deleted = 0
	`, compID).Scan(&mode)
	if err != nil {
		return "", err
	}
	return mode, nil
}

// GetAverageSolveTimeFromStart 获取平均解题时间（从比赛开始到首次正确提交的平均秒数）
func GetAverageSolveTimeFromStart(ctx context.Context, db DBTX, compID string) (float64, error) {
	var avgSolveTimeSec sql.NullFloat64
	err := db.QueryRowContext(ctx, `
		SELECT AVG(TIMESTAMPDIFF(SECOND, c.start_time, s.created_at))
		FROM submissions s
		JOIN competitions c ON c.res_id = s.competition_id
		WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0 AND c.is_deleted = 0
	`, compID).Scan(&avgSolveTimeSec)
	if err != nil {
		return 0, err
	}
	if avgSolveTimeSec.Valid {
		return avgSolveTimeSec.Float64, nil
	}
	return 0, nil
}

// CategoryStat 分类统计信息
type CategoryStat struct {
	Category          string // 分类名称
	TotalChallenges   int    // 该分类题目数
	TotalSolves       int    // 该分类总解题数
	UniqueUsersSolved int    // 该分类独立解题用户数
	TotalAttempts     int    // 该分类总提交数
}

// GetCategoryStats 获取比赛各分类的统计信息
func GetCategoryStats(ctx context.Context, db DBTX, compID string) ([]CategoryStat, error) {
	rows, err := db.QueryContext(ctx, `
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
		return nil, err
	}
	defer rows.Close()

	var stats []CategoryStat
	for rows.Next() {
		var cat CategoryStat
		if err := rows.Scan(&cat.Category, &cat.TotalChallenges, &cat.TotalSolves, &cat.UniqueUsersSolved, &cat.TotalAttempts); err != nil {
			return nil, err
		}
		stats = append(stats, cat)
	}
	return stats, rows.Err()
}

// UserFullStat 用户完整统计信息（包含提交次数和解题时间）
type UserFullStat struct {
	UserID         string
	TotalSolves    int
	TotalScore     int
	TotalAttempts  int
	FirstSolveAt   *time.Time
	LastSolveAt    *time.Time
}

// GetUserFullStats 获取比赛中所有用户的完整统计信息
func GetUserFullStats(ctx context.Context, db DBTX, compID string) ([]UserFullStat, error) {
	rows, err := db.QueryContext(ctx, `
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
		return nil, err
	}
	defer rows.Close()

	var stats []UserFullStat
	for rows.Next() {
		var u UserFullStat
		var firstSolve, lastSolve sql.NullTime
		if err := rows.Scan(&u.UserID, &u.TotalSolves, &u.TotalScore, &u.TotalAttempts, &firstSolve, &lastSolve); err != nil {
			return nil, err
		}
		if firstSolve.Valid {
			u.FirstSolveAt = &firstSolve.Time
		}
		if lastSolve.Valid {
			u.LastSolveAt = &lastSolve.Time
		}
		stats = append(stats, u)
	}
	return stats, rows.Err()
}

// ChallengeFullStat 题目完整统计信息
type ChallengeFullStat struct {
	ChallengeID       string
	TotalSolves       int
	TotalAttempts     int
	UniqueUsersSolved int
	FirstSolveAt      *time.Time
	AvgSolveTimeSec   float64 // 平均解题时间（从首次提交到正确提交的秒数）
}

// GetChallengeFullStats 获取比赛中各题目的完整统计信息
func GetChallengeFullStats(ctx context.Context, db DBTX, compID string) ([]ChallengeFullStat, error) {
	// 首先获取基础统计
	rows, err := db.QueryContext(ctx, `
		SELECT
			s.challenge_id,
			SUM(CASE WHEN s.is_correct = 1 THEN 1 ELSE 0 END) as total_solves,
			COUNT(*) as total_attempts,
			COUNT(DISTINCT CASE WHEN s.is_correct = 1 THEN s.user_id ELSE NULL END) as unique_users_solved,
			MIN(CASE WHEN s.is_correct = 1 THEN s.created_at ELSE NULL END) as first_solve
		FROM submissions s
		WHERE s.competition_id = ? AND s.is_deleted = 0
		GROUP BY s.challenge_id
	`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ChallengeFullStat
	for rows.Next() {
		var cs ChallengeFullStat
		var firstSolve sql.NullTime
		if err := rows.Scan(&cs.ChallengeID, &cs.TotalSolves, &cs.TotalAttempts, &cs.UniqueUsersSolved, &firstSolve); err != nil {
			return nil, err
		}
		if firstSolve.Valid {
			cs.FirstSolveAt = &firstSolve.Time
		}
		stats = append(stats, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 获取每道题的平均解题时间
	for i := range stats {
		avgTime, err := getChallengeAvgSolveTime(ctx, db, compID, stats[i].ChallengeID)
		if err == nil {
			stats[i].AvgSolveTimeSec = avgTime
		}
	}

	return stats, nil
}

// getChallengeAvgSolveTime 获取单题的平均解题时间（内部辅助函数）
func getChallengeAvgSolveTime(ctx context.Context, db DBTX, compID, challengeID string) (float64, error) {
	var avgSolveTime sql.NullFloat64
	err := db.QueryRowContext(ctx, `
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
	`, compID, challengeID).Scan(&avgSolveTime)
	if err != nil {
		return 0, err
	}
	if avgSolveTime.Valid {
		return avgSolveTime.Float64, nil
	}
	return 0, nil
}
