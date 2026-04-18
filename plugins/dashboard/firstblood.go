package dashboard

import (
	"context"
	"database/sql"
	"time"

	"ad7/internal/event"
	"ad7/internal/uuid"
)

// handleCorrectSubmission 处理正确提交事件。
// 判断该提交是否为某道题目的一血（首个正确提交）：
//  1. 查询数据库中是否已存在该题目的一血记录
//  2. 如果不存在，尝试使用 INSERT IGNORE 插入（处理并发竞争）
//  3. 插入成功则记录一血事件，失败则记录普通解题事件
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	ctx := context.Background()
	compID := ""
	if e.CompetitionID != nil {
		compID = *e.CompetitionID
	}

	// 检查数据库中是否已有一血记录
	var exists bool
	err := p.db.QueryRowContext(ctx, `
		SELECT 1 FROM dashboard_first_blood
		WHERE challenge_id = ? AND competition_id = ?
		LIMIT 1`, e.ChallengeID, compID).Scan(&exists)
	if err == nil {
		// 已有一血，添加普通解题事件
		p.addSolveEvent(ctx, e.UserID, e.ChallengeID)
		return
	}
	if err != sql.ErrNoRows {
		// 数据库错误，忽略（不影响主流程）
		return
	}

	// 尝试插入一血记录（INSERT IGNORE 处理并发竞争）
	resID := uuid.Next()

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
		// 并发导致插入失败（其他请求先插入了一血），记录普通解题事件
		p.addSolveEvent(ctx, e.UserID, e.ChallengeID)
		return
	}

	// 插入成功，这是一血！
	p.addFirstBloodEvent(ctx, e.UserID, e.ChallengeID, now)
}

// addFirstBloodEvent 添加一血事件到最近事件列表。
// 查询题目标题用于展示。
func (p *Plugin) addFirstBloodEvent(ctx context.Context, userID, challengeID string, t time.Time) {
	// 查询题目标题
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
	// 查询题目标题和分数
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
