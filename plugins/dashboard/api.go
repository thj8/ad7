package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (p *Plugin) getState(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if len(compID) != 32 {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	state, err := p.getCompetitionState(r.Context(), compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (p *Plugin) getFirstBlood(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if len(compID) != 32 {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	list, err := p.getFirstBloodList(r.Context(), compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
