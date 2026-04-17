package dashboard

import (
	"database/sql"
	"sync"

	"github.com/go-chi/chi/v5"

	"ad7/internal/event"
	"ad7/internal/middleware"
)

type Plugin struct {
	db           *sql.DB
	recentEvents []recentEvent
	mu           sync.RWMutex
}

func New() *Plugin {
	return &Plugin{
		recentEvents: make([]recentEvent, 0, 100),
	}
}

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db

	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	r.Get("/api/v1/dashboard/competitions/{id}/state", p.getState)
	r.Get("/api/v1/dashboard/competitions/{id}/firstblood", p.getFirstBlood)
}

func (p *Plugin) addRecentEvent(e recentEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recentEvents = append([]recentEvent{e}, p.recentEvents...)
	if len(p.recentEvents) > 100 {
		p.recentEvents = p.recentEvents[:100]
	}
}

func (p *Plugin) getRecentEvents() []recentEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]recentEvent, len(p.recentEvents))
	copy(result, p.recentEvents)
	return result
}
