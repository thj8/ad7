package topthree

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/event"
	"ad7/internal/middleware"
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

func (p *Plugin) handleCorrectSubmission(e event.Event) {
	if e.CompetitionID == nil || *e.CompetitionID == "" {
		return
	}
	compID := *e.CompetitionID
	chalID := e.ChallengeID
	userID := e.UserID
	submitTime := time.Now()

	ctx := context.Background()

	// Get current top three
	current, err := p.getCurrentTopThree(ctx, compID, chalID)
	if err != nil {
		return
	}

	// Use variables to avoid unused errors (will be used in future tasks)
	_ = chalID
	_ = userID
	_ = submitTime
	_ = current
}

func (p *Plugin) getTopThree(w http.ResponseWriter, r *http.Request) {
	// Will implement in later task
}
