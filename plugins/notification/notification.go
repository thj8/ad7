package notification

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

type Plugin struct{ db *sql.DB }

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate).Get("/api/v1/notifications", p.list)
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/notifications", p.createGlobal)
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/challenges/{id}/notifications", p.createForChallenge)
}

type notif struct {
	ID          int        `json:"id"`
	ChallengeID *int       `json:"challenge_id"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	CreatedAt   time.Time  `json:"created_at"`
}

type createReq struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func (p *Plugin) list(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	rows, err := p.db.QueryContext(r.Context(), `
		SELECT id, challenge_id, title, message, created_at
		FROM notifications
		WHERE challenge_id IS NULL
		   OR challenge_id IN (
		       SELECT DISTINCT challenge_id FROM submissions
		       WHERE user_id = ? AND is_correct = 1
		   )
		ORDER BY created_at DESC`, userID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ns []notif
	for rows.Next() {
		var n notif
		if err := rows.Scan(&n.ID, &n.ChallengeID, &n.Title, &n.Message, &n.CreatedAt); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		ns = append(ns, n)
	}
	if ns == nil {
		ns = []notif{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"notifications": ns})
}

func (p *Plugin) createGlobal(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	_, err := p.db.ExecContext(r.Context(),
		`INSERT INTO notifications (title, message) VALUES (?, ?)`,
		req.Title, req.Message)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (p *Plugin) createForChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	_, err = p.db.ExecContext(r.Context(),
		`INSERT INTO notifications (challenge_id, title, message) VALUES (?, ?, ?)`,
		id, req.Title, req.Message)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
