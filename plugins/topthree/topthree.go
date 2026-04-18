package topthree

import (
	"context"
	"database/sql"
	"time"

	"ad7/internal/event"
	"ad7/internal/snowflake"
)

type Plugin struct {
	db *sql.DB
}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db

	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	r.Group(func(r chi.Router) {
		r.Use(auth.Authenticate)
		r.Get("/api/v1/topthree/competitions/{id}", p.getTopThree)
	})
}

func (p *Plugin) getCurrentTopThree(ctx context.Context, compID, chalID string) ([]topThreeRecord, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, res_id, competition_id, challenge_id, user_id, rank, created_at
		FROM topthree_records
		WHERE competition_id = ? AND challenge_id = ?
		ORDER BY rank ASC
	`, compID, chalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []topThreeRecord
	for rows.Next() {
		var r topThreeRecord
		err := rows.Scan(&r.ID, &r.ResID, &r.CompetitionID, &r.ChallengeID, &r.UserID, &r.Rank, &r.CreatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func userInTopThree(current []topThreeRecord, userID string) bool {
	for _, r := range current {
		if r.UserID == userID {
			return true
		}
	}
	return false
}

func calculateNewRank(current []topThreeRecord, submitTime time.Time) int {
	if len(current) < 3 {
		return len(current) + 1
	}

	for i, r := range current {
		if submitTime.Before(r.CreatedAt) {
			return i + 1
		}
	}

	return 0
}

func (p *Plugin) updateTopThree(ctx context.Context, compID, chalID, userID string, newRank int, submitTime time.Time, current []topThreeRecord) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if newRank <= len(current) {
		if newRank <= 2 && len(current) >= 3 {
			_, err := tx.ExecContext(ctx, `
				DELETE FROM topthree_records
				WHERE competition_id = ? AND challenge_id = ? AND rank = 3
			`, compID, chalID)
			if err != nil {
				return err
			}
		}

		for i := len(current); i >= newRank; i-- {
			if i+1 > 3 {
				continue
			}
			_, err := tx.ExecContext(ctx, `
				UPDATE topthree_records
				SET rank = ?
				WHERE competition_id = ? AND challenge_id = ? AND rank = ?
			`, i+1, compID, chalID, i)
			if err != nil {
				return err
			}
		}
	}

	resID := snowflake.Next()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO topthree_records
		(res_id, competition_id, challenge_id, user_id, rank, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, resID, compID, chalID, userID, newRank, submitTime)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (p *Plugin) handleCorrectSubmission(e event.Event) {
	if e.CompetitionID == nil || *e.CompetitionID == "" {
		return
	}
	compID := *e.CompetitionID
	chalID := e.ChallengeID
	userID := e.UserID
	submitTime := time.Now()

	ctx := context.Background()

	current, err := p.getCurrentTopThree(ctx, compID, chalID)
	if err != nil {
		return
	}

	if userInTopThree(current, userID) {
		return
	}

	newRank := calculateNewRank(current, submitTime)
	if newRank == 0 {
		return
	}

	_ = p.updateTopThree(ctx, compID, chalID, userID, newRank, submitTime, current)
}
