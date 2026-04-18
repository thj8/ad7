package topthree

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"

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

func (p *Plugin) handleCorrectSubmission(e event.Event) {
	// Will implement in next task
}

func (p *Plugin) getTopThree(w http.ResponseWriter, r *http.Request) {
	// Will implement in later task
}
