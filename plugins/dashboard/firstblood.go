package dashboard

import (
	"context"
	"database/sql"
	"time"

	"ad7/internal/event"
	"ad7/internal/snowflake"
)

func (p *Plugin) handleCorrectSubmission(e event.Event) {
	ctx := context.Background()
	compID := ""
	if e.CompetitionID != nil {
		compID = *e.CompetitionID
	}

	// 检查是否已有一血
	var exists bool
	err := p.db.QueryRowContext(ctx, `
		SELECT 1 FROM dashboard_first_blood
		WHERE challenge_id = ? AND competition_id = ?
		LIMIT 1`, e.ChallengeID, compID).Scan(&exists)
	if err == nil {
		// 已有一血，添加普通 solve 事件
		p.addSolveEvent(ctx, e.UserID, e.ChallengeID, compID)
		return
	}
	if err != sql.ErrNoRows {
		// DB 错误，忽略
		return
	}

	// 尝试插入一血
	resID := snowflake.Next()

	now := time.Now()
	result, err := p.db.ExecContext(ctx, `
		INSERT IGNORE INTO dashboard_first_blood
		(res_id, competition_id, challenge_id, user_id, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		resID, compID, e.ChallengeID, e.UserID, now)
	if err != nil {
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// 插入失败（并发导致已有一血），添加普通 solve 事件
		p.addSolveEvent(ctx, e.UserID, e.ChallengeID, compID)
		return
	}

	// 插入成功，是一血！
	p.addFirstBloodEvent(ctx, e.UserID, e.ChallengeID, compID, now)
}

func (p *Plugin) addFirstBloodEvent(ctx context.Context, userID string, challengeID, compID string, t time.Time) {
	// 获取题目标题
	var title string
	err := p.db.QueryRowContext(ctx, `
		SELECT title FROM challenges WHERE res_id = ?`, challengeID).Scan(&title)
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

func (p *Plugin) addSolveEvent(ctx context.Context, userID string, challengeID, compID string) {
	// 获取题目标题和分数
	var title string
	var score int
	err := p.db.QueryRowContext(ctx, `
		SELECT title, score FROM challenges WHERE res_id = ?`, challengeID).Scan(&title, &score)
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
