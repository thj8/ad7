package hints

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/snowflake"
)

type Plugin struct{ db *sql.DB }

func New() *Plugin { return &Plugin{} }

type hint struct {
	ResID     int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type createReq struct {
	Content string `json:"content"`
}

type updateReq struct {
	Content   *string `json:"content"`
	IsVisible *bool   `json:"is_visible"`
}

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/challenges/{id}/hints", p.create)
	r.With(auth.Authenticate, auth.RequireAdmin).Put("/api/v1/admin/hints/{id}", p.update)
	r.With(auth.Authenticate, auth.RequireAdmin).Delete("/api/v1/admin/hints/{id}", p.delete)
	r.With(auth.Authenticate).Get("/api/v1/challenges/{id}/hints", p.list)
}

func (p *Plugin) create(w http.ResponseWriter, r *http.Request) {
	chalID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid challenge id"}`, http.StatusBadRequest)
		return
	}

	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" || len(req.Content) > 4096 {
		http.Error(w, `{"error":"content is required (max 4096 chars)"}`, http.StatusBadRequest)
		return
	}

	_, err = p.db.ExecContext(r.Context(),
		`INSERT INTO hints (res_id, challenge_id, content) VALUES (?, ?, ?)`,
		snowflake.Next(), chalID, req.Content)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (p *Plugin) update(w http.ResponseWriter, r *http.Request) {
	hintID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid hint id"}`, http.StatusBadRequest)
		return
	}

	var req updateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Validate content if provided
	if req.Content != nil && (len(*req.Content) == 0 || len(*req.Content) > 4096) {
		http.Error(w, `{"error":"content must be 1-4096 chars"}`, http.StatusBadRequest)
		return
	}

	// Get current values first
	var currentContent string
	var currentIsVisible bool
	err = p.db.QueryRowContext(r.Context(),
		`SELECT content, is_visible FROM hints WHERE res_id = ?`, hintID).
		Scan(&currentContent, &currentIsVisible)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	// Determine new values
	newContent := currentContent
	if req.Content != nil {
		newContent = *req.Content
	}
	newIsVisible := currentIsVisible
	if req.IsVisible != nil {
		newIsVisible = *req.IsVisible
	}

	// Update
	_, err = p.db.ExecContext(r.Context(),
		`UPDATE hints SET content = ?, is_visible = ? WHERE res_id = ?`,
		newContent, newIsVisible, hintID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (p *Plugin) delete(w http.ResponseWriter, r *http.Request) {
	hintID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid hint id"}`, http.StatusBadRequest)
		return
	}

	result, err := p.db.ExecContext(r.Context(),
		`DELETE FROM hints WHERE res_id = ?`, hintID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (p *Plugin) list(w http.ResponseWriter, r *http.Request) {
	chalID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid challenge id"}`, http.StatusBadRequest)
		return
	}

	rows, err := p.db.QueryContext(r.Context(),
		`SELECT res_id, content, created_at FROM hints
		 WHERE challenge_id = ? AND is_visible = 1
		 ORDER BY created_at ASC`, chalID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hints []hint
	for rows.Next() {
		var h hint
		if err := rows.Scan(&h.ResID, &h.Content, &h.CreatedAt); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		hints = append(hints, h)
	}

	if hints == nil {
		hints = []hint{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"hints": hints})
}
