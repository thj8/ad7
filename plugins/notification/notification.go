package notification

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/model"
	"ad7/internal/snowflake"
)

type Plugin struct{ db *sql.DB }

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/competitions/{id}/notifications", p.createForComp)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/notifications", p.listByComp)
}

type createReq struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func (p *Plugin) listByComp(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	rows, err := p.db.QueryContext(r.Context(), `
		SELECT res_id, competition_id, title, message, created_at
		FROM notifications
		WHERE competition_id = ? AND is_deleted = 0
		ORDER BY created_at DESC`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ns []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ResID, &n.CompetitionID, &n.Title, &n.Message, &n.CreatedAt); err != nil {
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
	compID := chi.URLParam(r, "id")
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.Message == "" {
		http.Error(w, `{"error":"title and message are required"}`, http.StatusBadRequest)
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
