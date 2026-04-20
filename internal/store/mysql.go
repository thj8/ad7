package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ad7/internal/model"
	"ad7/internal/uuid"
	_ "github.com/go-sql-driver/mysql"
)

// Store 是所有 Store 接口的统一实现，持有 MySQL 数据库连接。
// 单一结构体实现了 ChallengeStore、SubmissionStore、CompetitionStore 三个接口。
type Store struct {
	db *sql.DB
}

// New 创建新的 Store 实例，连接到指定的 MySQL 数据库。
// 参数 dsn 为 MySQL 数据源名称，格式：user:password@tcp(host:port)/dbname?parseTime=true
// 返回初始化后的 Store 实例，连接时会执行 Ping 验证数据库可用性。
func New(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// 验证数据库连接是否可用
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error { return s.db.Close() }

// DB 返回底层的 *sql.DB 实例，供插件直接使用。
func (s *Store) DB() *sql.DB  { return s.db }

// ListEnabled 查询所有已启用且未删除的题目（不含 Flag）。
// 返回题目列表，按查询顺序排列。
func (s *Store) ListEnabled(ctx context.Context) ([]model.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT res_id, title, category, description, score, is_enabled, created_at, updated_at
		 FROM challenges WHERE is_enabled = 1 AND is_deleted = 0`)
	if err != nil {
		return nil, fmt.Errorf("list enabled challenges: %w", err)
	}
	defer rows.Close()
	var cs []model.Challenge
	for rows.Next() {
		var c model.Challenge
		if err := rows.Scan(&c.ResID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan enabled challenge: %w", err)
		}
		cs = append(cs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled challenges: %w", err)
	}
	return cs, nil
}

// GetEnabledByID 根据 res_id 查询单个已启用且未删除的题目（含 Flag）。
// 用于提交 Flag 时验证答案。如果未找到返回 nil, nil。
func (s *Store) GetEnabledByID(ctx context.Context, resID string) (*model.Challenge, error) {
	var c model.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, title, category, description, score, flag, is_enabled, created_at, updated_at
		 FROM challenges WHERE res_id = ? AND is_enabled = 1 AND is_deleted = 0`, resID).
		Scan(&c.ResID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.Flag, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &c, fmt.Errorf("get enabled challenge by id %s: %w", resID, err)
}

// GetByID 根据 res_id 查询单个未删除的题目（含 Flag），不检查启用状态。
// 用于管理员更新题目时需要获取完整信息（包括未启用的题目）。
func (s *Store) GetByID(ctx context.Context, resID string) (*model.Challenge, error) {
	var c model.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, title, category, description, score, flag, is_enabled, created_at, updated_at
		 FROM challenges WHERE res_id = ? AND is_deleted = 0`, resID).
		Scan(&c.ResID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.Flag, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &c, fmt.Errorf("get challenge by id %s: %w", resID, err)
}

// Create 创建新题目。自动生成 32 字符 UUID 作为 res_id。
// 返回生成的 res_id。
func (s *Store) Create(ctx context.Context, c *model.Challenge) (string, error) {
	c.ResID = uuid.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO challenges (res_id, title, category, description, score, flag, is_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ResID, c.Title, c.Category, c.Description, c.Score, c.Flag, c.IsEnabled)
	if err != nil {
		return "", fmt.Errorf("create challenge: %w", err)
	}
	return c.ResID, nil
}

// Update 根据 res_id 更新题目的全部字段（title, category, description, score, flag, is_enabled）。
func (s *Store) Update(ctx context.Context, c *model.Challenge) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE challenges SET title=?, category=?, description=?, score=?, flag=?, is_enabled=? WHERE res_id=? AND is_deleted = 0`,
		c.Title, c.Category, c.Description, c.Score, c.Flag, c.IsEnabled, c.ResID)
	return fmt.Errorf("update challenge %s: %w", c.ResID, err)
}

// Delete 软删除题目，将 is_deleted 字段设为 1。
// 已删除的题目不会出现在查询结果中。
func (s *Store) Delete(ctx context.Context, resID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE challenges SET is_deleted=1 WHERE res_id = ? AND is_deleted = 0`, resID)
	return fmt.Errorf("delete challenge %s: %w", resID, err)
}

// HasCorrectSubmission 检查指定用户在指定比赛中是否已正确提交过某道题目。
// 用于防止重复提交，competitionID 为必填参数。
func (s *Store) HasCorrectSubmission(ctx context.Context, userID string, challengeID string, competitionID string) (bool, error) {
	query := `SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND competition_id=? AND is_correct=1 AND is_deleted=0`
	var count int
	err := s.db.QueryRowContext(ctx, query, userID, challengeID, competitionID).Scan(&count)
	return count > 0, err
}

// CreateSubmission 创建提交记录。
// CompetitionID 为必填字段，所有提交都关联到比赛。
func (s *Store) CreateSubmission(ctx context.Context, sub *model.Submission) error {
	sub.ResID = uuid.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO submissions (res_id, user_id, challenge_id, competition_id, submitted_flag, is_correct) VALUES (?, ?, ?, ?, ?, ?)`,
		sub.ResID, sub.UserID, sub.ChallengeID, sub.CompetitionID, sub.SubmittedFlag, sub.IsCorrect)
	return fmt.Errorf("create submission for user %s, challenge %s: %w", sub.UserID, sub.ChallengeID, err)
}

// ListSubmissions 根据 params 查询提交记录。
// 按创建时间倒序排列。动态拼接 SQL 条件。
func (s *Store) ListSubmissions(ctx context.Context, params ListSubmissionsParams) ([]model.Submission, error) {
	query := `SELECT res_id, user_id, challenge_id, competition_id, submitted_flag, is_correct, created_at FROM submissions WHERE is_deleted=0`
	args := []any{}
	if params.CompetitionID != "" {
		query += " AND competition_id=?"
		args = append(args, params.CompetitionID)
	}
	if params.UserID != "" {
		query += " AND user_id=?"
		args = append(args, params.UserID)
	}
	if params.ChallengeID != "" {
		query += " AND challenge_id=?"
		args = append(args, params.ChallengeID)
	}
	query += " ORDER BY created_at DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []model.Submission
	for rows.Next() {
		var sub model.Submission
		if err := rows.Scan(&sub.ResID, &sub.UserID, &sub.ChallengeID, &sub.CompetitionID,
			&sub.SubmittedFlag, &sub.IsCorrect, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// --- CompetitionStore 接口实现 ---

// ListCompetitions 查询所有未删除的比赛，按创建时间倒序排列。
// 管理员接口，包含未激活的比赛。
func (s *Store) ListCompetitions(ctx context.Context) ([]model.Competition, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT res_id, title, description, start_time, end_time, is_active, created_at, updated_at
		 FROM competitions WHERE is_deleted = 0 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []model.Competition
	for rows.Next() {
		var c model.Competition
		if err := rows.Scan(&c.ResID, &c.Title, &c.Description, &c.StartTime, &c.EndTime, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

// ListActiveCompetitions 查询所有已激活且未删除的比赛，按创建时间倒序排列。
// 普通用户接口，只展示激活中的比赛。
func (s *Store) ListActiveCompetitions(ctx context.Context) ([]model.Competition, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT res_id, title, description, start_time, end_time, is_active, created_at, updated_at
		 FROM competitions WHERE is_active = 1 AND is_deleted = 0 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []model.Competition
	for rows.Next() {
		var c model.Competition
		if err := rows.Scan(&c.ResID, &c.Title, &c.Description, &c.StartTime, &c.EndTime, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

// GetCompetitionByID 根据 res_id 查询单个未删除的比赛。
// 如果未找到返回 nil, nil。
func (s *Store) GetCompetitionByID(ctx context.Context, resID string) (*model.Competition, error) {
	var c model.Competition
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, title, description, start_time, end_time, is_active, created_at, updated_at
		 FROM competitions WHERE res_id = ? AND is_deleted = 0`, resID).
		Scan(&c.ResID, &c.Title, &c.Description, &c.StartTime, &c.EndTime, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &c, fmt.Errorf("get competition by id %s: %w", resID, err)
}

// CreateCompetition 创建新比赛，自动生成 res_id。
// 返回生成的 res_id。
func (s *Store) CreateCompetition(ctx context.Context, c *model.Competition) (string, error) {
	c.ResID = uuid.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO competitions (res_id, title, description, start_time, end_time, is_active) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ResID, c.Title, c.Description, c.StartTime, c.EndTime, c.IsActive)
	if err != nil {
		return "", err
	}
	return c.ResID, nil
}

// UpdateCompetition 根据 res_id 更新比赛信息。
func (s *Store) UpdateCompetition(ctx context.Context, c *model.Competition) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE competitions SET title=?, description=?, start_time=?, end_time=?, is_active=? WHERE res_id=? AND is_deleted = 0`,
		c.Title, c.Description, c.StartTime, c.EndTime, c.IsActive, c.ResID)
	return fmt.Errorf("update competition %s: %w", c.ResID, err)
}

// DeleteCompetition 软删除比赛。先删除该比赛的题目关联记录，再将比赛标记为已删除。
func (s *Store) DeleteCompetition(ctx context.Context, resID string) error {
	// 软删除比赛与题目的关联记录
	if _, err := s.db.ExecContext(ctx,
		`UPDATE competition_challenges
		 SET is_deleted = 1, deleted_at = NOW()
		 WHERE competition_id = ? AND deleted_at IS NULL`, resID); err != nil {
		return fmt.Errorf("soft delete competition challenges for %s: %w", resID, err)
	}
	// 软删除比赛本身
	_, err := s.db.ExecContext(ctx, `UPDATE competitions SET is_deleted=1 WHERE res_id = ? AND is_deleted = 0`, resID)
	return fmt.Errorf("delete competition %s: %w", resID, err)
}

// AddChallenge 将一道题目分配到比赛中，自动生成关联记录的 res_id。
func (s *Store) AddChallenge(ctx context.Context, compID, chalID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO competition_challenges (res_id, competition_id, challenge_id) VALUES (?, ?, ?)`,
		uuid.Next(), compID, chalID)
	return fmt.Errorf("error: %w", err)
}

// RemoveChallenge 从比赛中移除一道题目的关联记录（软删除）。
func (s *Store) RemoveChallenge(ctx context.Context, compID, chalID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE competition_challenges
		 SET is_deleted = 1, deleted_at = NOW()
		 WHERE competition_id = ? AND challenge_id = ? AND deleted_at IS NULL`,
		compID, chalID)
	return fmt.Errorf("soft remove challenge: %w", err)
}

// ListCompChallenges 查询指定比赛中所有已启用且未删除的题目。
// 通过 competition_challenges 关联表 JOIN challenges 表查询，不包含 Flag 字段。
func (s *Store) ListCompChallenges(ctx context.Context, compID string) ([]model.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.res_id, c.title, c.category, c.description, c.score, c.is_enabled, c.created_at, c.updated_at
		 FROM challenges c
		 JOIN competition_challenges cc ON cc.challenge_id = c.res_id
		 WHERE cc.competition_id = ?
		   AND cc.is_deleted = 0
		   AND cc.deleted_at IS NULL
		   AND c.is_enabled = 1
		   AND c.is_deleted = 0`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []model.Challenge
	for rows.Next() {
		var c model.Challenge
		if err := rows.Scan(&c.ResID, &c.Title, &c.Category, &c.Description, &c.Score, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

// SetActive 根据 res_id 设置比赛的 is_active 状态。
// WHERE 条件包含 is_deleted = 0，不会修改已删除的比赛。
func (s *Store) SetActive(ctx context.Context, resID string, active bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE competitions SET is_active = ? WHERE res_id = ? AND is_deleted = 0`,
		active, resID)
	return fmt.Errorf("error: %w", err)
}

