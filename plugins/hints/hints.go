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
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) delete(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) list(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}
