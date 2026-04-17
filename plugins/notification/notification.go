package notification

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/model"
	"ad7/internal/snowflake"
)

type Plugin struct{ db *sql.DB }

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate).Get("/api/v1/notifications", p.list)
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/notifications", p.createGlobal)
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/challenges/{id}/notifications", p.createForChallenge)
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/competitions/{id}/notifications", p.createForComp)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/notifications", p.listByComp)
}

type createReq struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func (p *Plugin) list(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	rows, err := p.db.QueryContext(r.Context(), `
		SELECT res_id, competition_id, challenge_id, title, message, created_at
		FROM notifications
		WHERE competition_id IS NULL
		  AND (challenge_id IS NULL
		   OR challenge_id IN (
		       SELECT DISTINCT challenge_id FROM submissions
		       WHERE user_id = ? AND is_correct = 1
		   ))
		ORDER BY created_at DESC`, userID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ns []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ResID, &n.CompetitionID, &n.ChallengeID, &n.Title, &n.Message, &n.CreatedAt); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		ns = append(ns, n)
	}
	if ns == nil {
		ns = []model.Notification{}
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
		`INSERT INTO notifications (res_id, title, message) VALUES (?, ?, ?)`,
		snowflake.Next(), req.Title, req.Message)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (p *Plugin) createForChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
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
		`INSERT INTO notifications (res_id, challenge_id, title, message) VALUES (?, ?, ?, ?)`,
		snowflake.Next(), id, req.Title, req.Message)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (p *Plugin) listByComp(w http.ResponseWriter, r *http.Request) {
	compID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	rows, err := p.db.QueryContext(r.Context(), `
		SELECT res_id, competition_id, challenge_id, title, message, created_at
		FROM notifications
		WHERE competition_id = ?
		ORDER BY created_at DESC`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ns []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ResID, &n.CompetitionID, &n.ChallengeID, &n.Title, &n.Message, &n.CreatedAt); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		ns = append(ns, n)
	}
	if ns == nil {
		ns = []model.Notification{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"notifications": ns})
}

func (p *Plugin) createForComp(w http.ResponseWriter, r *http.Request) {
	compID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	_, err := p.db.ExecContext(r.Context(),
		`INSERT INTO notifications (res_id, competition_id, title, message) VALUES (?, ?, ?, ?)`,
		snowflake.Next(), compID, req.Title, req.Message)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
