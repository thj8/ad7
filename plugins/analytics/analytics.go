package analytics

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
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/overview", p.overview)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/categories", p.byCategory)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/users", p.userStats)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/challenges", p.challengeStats)
}

func (p *Plugin) overview(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) byCategory(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) userStats(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) challengeStats(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}
