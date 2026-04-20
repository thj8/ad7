// Package topthree 实现三血（前三名正确提交者）追踪插件。
// 当用户在比赛中正确提交 Flag 时，通过事件系统异步检测并记录三血排名。
// 排名基于提交时间：越早提交排名越靠前。
package topthree

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/event"
	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/uuid"
)

// Plugin 是三血追踪插件，持有数据库连接。
type Plugin struct {
	db *sql.DB
}

// New 创建三血插件实例。
func New() *Plugin {
	return &Plugin{}
}

// Name 返回插件名称
func (p *Plugin) Name() string {
	return plugin.NameTopThree
}

// Register 注册三血插件的路由并订阅正确提交事件。
// 路由：GET /api/v1/topthree/competitions/{id}（需要认证）
// 同时订阅 EventCorrectSubmission 事件，用于实时更新三血排名。
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db

	// 订阅正确提交事件，触发三血排名更新
	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	r.Group(func(r chi.Router) {
		r.Use(auth.Authenticate)
		r.Get("/api/v1/topthree/competitions/{id}", p.getTopThree)
	})
}

// getCurrentTopThreeForUpdate 在事务中查询指定比赛中某道题目的当前三血记录。
// 使用 SELECT ... FOR UPDATE 加行锁，防止并发修改导致竞态条件。
// 返回排名 1-3 的记录（按排名升序）。
func getCurrentTopThreeForUpdate(ctx context.Context, tx *sql.Tx, compID, chalID string) ([]topThreeRecord, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, res_id, competition_id, challenge_id, user_id, ranking, created_at, updated_at, is_deleted
		FROM topthree_records
		WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 0
		ORDER BY ranking ASC
		FOR UPDATE
	`, compID, chalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []topThreeRecord
	for rows.Next() {
		var r topThreeRecord
		err := rows.Scan(&r.ID, &r.ResID, &r.CompetitionID, &r.ChallengeID, &r.UserID, &r.Ranking, &r.CreatedAt, &r.UpdatedAt, &r.IsDeleted)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// userInTopThree 检查用户是否已在三血名单中。
// 如果已在名单中则跳过（不允许重复上榜）。
func userInTopThree(current []topThreeRecord, userID string) bool {
	for _, r := range current {
		if r.UserID == userID {
			return true
		}
	}
	return false
}

// calculateNewRank 计算新提交应该插入的排名位置。
// 如果当前三血已满（3 人）且新提交时间晚于所有现有记录，返回 0（不入榜）。
// 否则返回 1-3 之间的排名值。
func calculateNewRank(current []topThreeRecord, submitTime time.Time) int {
	// 三血未满，直接追加
	if len(current) < 3 {
		return len(current) + 1
	}

	// 三血已满，检查是否能替换（比现有记录更早）
	for i, r := range current {
		if submitTime.Before(r.CreatedAt) {
			return i + 1
		}
	}

	// 比所有人都晚，不入榜
	return 0
}

// updateTopThreeRequest 是更新三血排名的请求参数。
type updateTopThreeRequest struct {
	CompID     string // 比赛ID
	ChalID     string // 题目ID
	UserID     string // 用户ID
	NewRank    int    // 新排名（1-3）
	SubmitTime time.Time // 提交时间
	Current    []topThreeRecord // 当前三血记录
}

// updateTopThreeInTx 在已有事务中更新三血排名。
// 如果新排名在第 1 或第 2 位：
//   - 原第 3 名被软删除
//   - 比新排名低的原有记录排名 +1
//
// 然后插入新的排名记录。
func updateTopThreeInTx(ctx context.Context, tx *sql.Tx, req *updateTopThreeRequest) error {
	// 如果插入位置在已有记录范围内，需要移动或删除现有记录
	if req.NewRank <= len(req.Current) {
		// 如果新排名是第 1 或第 2，且当前已满 3 人，则软删除第 3 名
		if req.NewRank <= 2 && len(req.Current) >= 3 {
			_, err := tx.ExecContext(ctx, `
				UPDATE topthree_records
				SET is_deleted = 1, ranking = 0, updated_at = NOW()
				WHERE competition_id = ? AND challenge_id = ? AND ranking = 3 AND is_deleted = 0
			`, req.CompID, req.ChalID)
			if err != nil {
				return err
			}
		}

		// 将排名 >= NewRank 的记录排名 +1（从后往前更新避免冲突）
		for i := len(req.Current); i >= req.NewRank; i-- {
			if i+1 > 3 {
				continue // 超出三血范围的忽略
			}
			_, err := tx.ExecContext(ctx, `
				UPDATE topthree_records
				SET ranking = ?
				WHERE competition_id = ? AND challenge_id = ? AND ranking = ?
			`, i+1, req.CompID, req.ChalID, i)
			if err != nil {
				return err
			}
		}
	}

	// 插入新的三血记录
	resID := uuid.Next()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO topthree_records
		(res_id, competition_id, challenge_id, user_id, ranking, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, resID, req.CompID, req.ChalID, req.UserID, req.NewRank, req.SubmitTime)
	return err
}

// handleCorrectSubmission 处理正确提交事件，更新三血排名。
// 在单个事务中完成读取和写入，避免并发竞态条件。
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	compID := e.CompetitionID
	chalID := e.ChallengeID
	userID := e.UserID
	submitTime := e.SubmittedAt

	ctx := context.Background()

	// 开启事务，将读取和写入放在同一事务中
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[topthree] 开启事务失败: comp=%s chal=%s user=%s err=%v", compID, chalID, userID, err)
		return
	}
	defer tx.Rollback()

	// 在事务中加锁读取当前三血记录
	current, err := getCurrentTopThreeForUpdate(ctx, tx, compID, chalID)
	if err != nil {
		log.Printf("[topthree] 查询三血记录失败: comp=%s chal=%s user=%s err=%v", compID, chalID, userID, err)
		return
	}

	// 用户已上榜则跳过
	if userInTopThree(current, userID) {
		return
	}

	// 计算新排名
	newRank := calculateNewRank(current, submitTime)
	if newRank == 0 {
		return // 不入榜
	}

	// 在同一事务中更新三血排名
	if err := updateTopThreeInTx(ctx, tx, &updateTopThreeRequest{
		CompID:     compID,
		ChalID:     chalID,
		UserID:     userID,
		NewRank:    newRank,
		SubmitTime: submitTime,
		Current:    current,
	}); err != nil {
		log.Printf("[topthree] 更新三血排名失败: comp=%s chal=%s user=%s rank=%d err=%v", compID, chalID, userID, newRank, err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[topthree] 提交事务失败: comp=%s chal=%s user=%s err=%v", compID, chalID, userID, err)
	}
}

// GetBloodRank 获取用户在某道题目的三血排名
// 返回值: 1=一血, 2=二血, 3=三血, 0=未入榜, -1=查询错误
func (p *Plugin) GetBloodRank(ctx context.Context, compID, chalID, userID string) (int, error) {
	var ranking int
	err := p.db.QueryRowContext(ctx, `
		SELECT ranking FROM topthree_records
		WHERE competition_id = ? AND challenge_id = ? AND user_id = ? AND is_deleted = 0
		LIMIT 1
	`, compID, chalID, userID).Scan(&ranking)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return -1, err
	}
	return ranking, nil
}

// GetCompTopThree 获取比赛每道题目的三血信息
// 返回值: map[challengeID]BloodRankEntry
func (p *Plugin) GetCompTopThree(ctx context.Context, compID string) (map[string]BloodRankEntry, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT challenge_id, user_id, ranking
		FROM topthree_records
		WHERE competition_id = ? AND is_deleted = 0 AND ranking <= 3
		ORDER BY ranking ASC
	`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]BloodRankEntry)
	for rows.Next() {
		var chalID, userID string
		var ranking int
		if err := rows.Scan(&chalID, &userID, &ranking); err != nil {
			return nil, err
		}
		entry := result[chalID]
		entry.ChallengeID = chalID
		switch ranking {
		case 1:
			entry.FirstBlood = userID
		case 2:
			entry.SecondBlood = userID
		case 3:
			entry.ThirdBlood = userID
		}
		result[chalID] = entry
	}
	return result, rows.Err()
}
