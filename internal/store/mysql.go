package store

import (
	"context"
	"database/sql"
	"fmt"

	"ad7/internal/model"
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

func (s *Store) Close() error  { return s.db.Close() }
func (s *Store) DB() *sql.DB   { return s.db }

func (s *Store) ListEnabled(ctx context.Context) ([]model.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, category, description, score, is_enabled, created_at, updated_at
		 FROM challenges WHERE is_enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []model.Challenge
	for rows.Next() {
		var c model.Challenge
		if err := rows.Scan(&c.ID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

func (s *Store) GetEnabledByID(ctx context.Context, id int) (*model.Challenge, error) {
	var c model.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, category, description, score, is_enabled, created_at, updated_at
		 FROM challenges WHERE id = ? AND is_enabled = 1`, id).
		Scan(&c.ID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (s *Store) GetByID(ctx context.Context, id int) (*model.Challenge, error) {
	var c model.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, category, description, score, flag, is_enabled, created_at, updated_at
		 FROM challenges WHERE id = ?`, id).
		Scan(&c.ID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.Flag, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (s *Store) Create(ctx context.Context, c *model.Challenge) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO challenges (title, category, description, score, flag, is_enabled)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.Title, c.Category, c.Description, c.Score, c.Flag, c.IsEnabled)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) Update(ctx context.Context, c *model.Challenge) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE challenges SET title=?, category=?, description=?, score=?, flag=?, is_enabled=? WHERE id=?`,
		c.Title, c.Category, c.Description, c.Score, c.Flag, c.IsEnabled, c.ID)
	return err
}

func (s *Store) Delete(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM challenges WHERE id = ?`, id)
	return err
}

func (s *Store) HasCorrectSubmission(ctx context.Context, userID string, challengeID int) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct=1`,
		userID, challengeID).Scan(&count)
	return count > 0, err
}

func (s *Store) CreateSubmission(ctx context.Context, sub *model.Submission) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO submissions (user_id, challenge_id, submitted_flag, is_correct) VALUES (?, ?, ?, ?)`,
		sub.UserID, sub.ChallengeID, sub.SubmittedFlag, sub.IsCorrect)
	return err
}

func (s *Store) ListSubmissions(ctx context.Context, userID string, challengeID int) ([]model.Submission, error) {
	query := `SELECT id, user_id, challenge_id, submitted_flag, is_correct, created_at FROM submissions WHERE 1=1`
	args := []any{}
	if userID != "" {
		query += " AND user_id=?"
		args = append(args, userID)
	}
	if challengeID > 0 {
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
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.ChallengeID,
			&sub.SubmittedFlag, &sub.IsCorrect, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}
