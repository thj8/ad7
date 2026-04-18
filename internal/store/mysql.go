package store

import (
	"context"
	"database/sql"
	"fmt"

	"ad7/internal/model"
	"ad7/internal/snowflake"
	_ "github.com/go-sql-driver/mysql"
)

type Store struct {
	db *sql.DB
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) DB() *sql.DB  { return s.db }

func (s *Store) ListEnabled(ctx context.Context) ([]model.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT res_id, title, category, description, score, is_enabled, created_at, updated_at
		 FROM challenges WHERE is_enabled = 1 AND is_deleted = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []model.Challenge
	for rows.Next() {
		var c model.Challenge
		if err := rows.Scan(&c.ResID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

func (s *Store) GetEnabledByID(ctx context.Context, resID string) (*model.Challenge, error) {
	var c model.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, title, category, description, score, flag, is_enabled, created_at, updated_at
		 FROM challenges WHERE res_id = ? AND is_enabled = 1 AND is_deleted = 0`, resID).
		Scan(&c.ResID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.Flag, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (s *Store) GetByID(ctx context.Context, resID string) (*model.Challenge, error) {
	var c model.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, title, category, description, score, flag, is_enabled, created_at, updated_at
		 FROM challenges WHERE res_id = ? AND is_deleted = 0`, resID).
		Scan(&c.ResID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.Flag, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (s *Store) Create(ctx context.Context, c *model.Challenge) (string, error) {
	c.ResID = snowflake.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO challenges (res_id, title, category, description, score, flag, is_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ResID, c.Title, c.Category, c.Description, c.Score, c.Flag, c.IsEnabled)
	if err != nil {
		return "", err
	}
	return c.ResID, nil
}

func (s *Store) Update(ctx context.Context, c *model.Challenge) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE challenges SET title=?, category=?, description=?, score=?, flag=?, is_enabled=? WHERE res_id=?`,
		c.Title, c.Category, c.Description, c.Score, c.Flag, c.IsEnabled, c.ResID)
	return err
}

func (s *Store) Delete(ctx context.Context, resID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE challenges SET is_deleted=1 WHERE res_id = ?`, resID)
	return err
}

func (s *Store) HasCorrectSubmission(ctx context.Context, userID string, challengeID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct=1`,
		userID, challengeID).Scan(&count)
	return count > 0, err
}

func (s *Store) CreateSubmission(ctx context.Context, sub *model.Submission) error {
	sub.ResID = snowflake.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO submissions (res_id, user_id, challenge_id, submitted_flag, is_correct) VALUES (?, ?, ?, ?, ?)`,
		sub.ResID, sub.UserID, sub.ChallengeID, sub.SubmittedFlag, sub.IsCorrect)
	return err
}

func (s *Store) ListSubmissions(ctx context.Context, userID string, challengeID string) ([]model.Submission, error) {
	query := `SELECT res_id, user_id, challenge_id, competition_id, submitted_flag, is_correct, created_at FROM submissions WHERE 1=1`
	args := []any{}
	if userID != "" {
		query += " AND user_id=?"
		args = append(args, userID)
	}
	if challengeID != "" {
		query += " AND challenge_id=?"
		args = append(args, challengeID)
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

// --- CompetitionStore implementation ---

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

func (s *Store) GetCompetitionByID(ctx context.Context, resID string) (*model.Competition, error) {
	var c model.Competition
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, title, description, start_time, end_time, is_active, created_at, updated_at
		 FROM competitions WHERE res_id = ? AND is_deleted = 0`, resID).
		Scan(&c.ResID, &c.Title, &c.Description, &c.StartTime, &c.EndTime, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (s *Store) CreateCompetition(ctx context.Context, c *model.Competition) (string, error) {
	c.ResID = snowflake.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO competitions (res_id, title, description, start_time, end_time, is_active) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ResID, c.Title, c.Description, c.StartTime, c.EndTime, c.IsActive)
	if err != nil {
		return "", err
	}
	return c.ResID, nil
}

func (s *Store) UpdateCompetition(ctx context.Context, c *model.Competition) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE competitions SET title=?, description=?, start_time=?, end_time=?, is_active=? WHERE res_id=?`,
		c.Title, c.Description, c.StartTime, c.EndTime, c.IsActive, c.ResID)
	return err
}

func (s *Store) DeleteCompetition(ctx context.Context, resID string) error {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM competition_challenges WHERE competition_id = ?`, resID)
	_, err := s.db.ExecContext(ctx, `UPDATE competitions SET is_deleted=1 WHERE res_id = ?`, resID)
	return err
}

func (s *Store) AddChallenge(ctx context.Context, compID, chalID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO competition_challenges (res_id, competition_id, challenge_id) VALUES (?, ?, ?)`,
		snowflake.Next(), compID, chalID)
	return err
}

func (s *Store) RemoveChallenge(ctx context.Context, compID, chalID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM competition_challenges WHERE competition_id = ? AND challenge_id = ?`,
		compID, chalID)
	return err
}

func (s *Store) ListCompChallenges(ctx context.Context, compID string) ([]model.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.res_id, c.title, c.category, c.description, c.score, c.is_enabled, c.created_at, c.updated_at
		 FROM challenges c
		 JOIN competition_challenges cc ON cc.challenge_id = c.res_id
		 WHERE cc.competition_id = ? AND c.is_enabled = 1 AND c.is_deleted = 0`, compID)
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

// --- Extended SubmissionStore methods ---

func (s *Store) HasCorrectSubmissionInComp(ctx context.Context, userID string, challengeID, competitionID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND competition_id=? AND is_correct=1`,
		userID, challengeID, competitionID).Scan(&count)
	return count > 0, err
}

func (s *Store) CreateSubmissionWithComp(ctx context.Context, sub *model.Submission) error {
	sub.ResID = snowflake.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO submissions (res_id, user_id, challenge_id, competition_id, submitted_flag, is_correct) VALUES (?, ?, ?, ?, ?, ?)`,
		sub.ResID, sub.UserID, sub.ChallengeID, sub.CompetitionID, sub.SubmittedFlag, sub.IsCorrect)
	return err
}

func (s *Store) ListSubmissionsByComp(ctx context.Context, competitionID string, userID string, challengeID string) ([]model.Submission, error) {
	query := `SELECT res_id, user_id, challenge_id, competition_id, submitted_flag, is_correct, created_at FROM submissions WHERE competition_id=?`
	args := []any{competitionID}
	if userID != "" {
		query += " AND user_id=?"
		args = append(args, userID)
	}
	if challengeID != "" {
		query += " AND challenge_id=?"
		args = append(args, challengeID)
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
