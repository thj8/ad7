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

// GetCompChallenges 获取比赛中所有已启用且未删除的题目摘要信息。
// 按题目 res_id 排序。调用方只需 ID 列表时，从返回值中提取 .ResID 即可。
func GetCompChallenges(ctx context.Context, db DBTX, compID string) ([]ChallengeInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.res_id, c.title, c.category, c.score
		FROM competition_challenges cc
		JOIN challenges c ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ?
		  AND cc.is_deleted = 0
		  AND cc.deleted_at IS NULL
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

// GetCompSubmitStats 获取比赛的总提交数和正确提交数。
// 返回 (总数, 正确数, 错误)。
func GetCompSubmitStats(ctx context.Context, db DBTX, compID string) (int, int, error) {
	var total, correct int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*), SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END)
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
		SELECT COUNT(*) FROM competition_challenges WHERE competition_id = ? AND is_deleted = 0 AND deleted_at IS NULL
	`, compID).Scan(&count)
	return count, err
}
