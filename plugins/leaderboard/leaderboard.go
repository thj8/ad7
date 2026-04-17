package leaderboard

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

type Plugin struct{ db *sql.DB }

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate).Get("/api/v1/leaderboard", p.list)
}

type entry struct {
	Rank          int       `json:"rank"`
	UserID        string    `json:"user_id"`
	TotalScore    int       `json:"total_score"`
	LastSolveTime time.Time `json:"last_solve_time"`
}

func (p *Plugin) list(w http.ResponseWriter, r *http.Request) {
	rows, err := p.db.QueryContext(r.Context(), `
		SELECT s.user_id, SUM(c.score), MAX(s.created_at)
		FROM submissions s
		JOIN challenges c ON c.id = s.challenge_id
		WHERE s.is_correct = 1
		GROUP BY s.user_id
		ORDER BY SUM(c.score) DESC, MAX(s.created_at) ASC`)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var board []entry
	rank := 1
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.UserID, &e.TotalScore, &e.LastSolveTime); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		e.Rank = rank
		rank++
		board = append(board, e)
	}
	if board == nil {
		board = []entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"leaderboard": board})
}
