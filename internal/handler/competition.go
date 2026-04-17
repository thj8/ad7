package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/model"
	"ad7/internal/service"
)

type CompetitionHandler struct {
	svc *service.CompetitionService
}

func NewCompetitionHandler(svc *service.CompetitionService) *CompetitionHandler {
	return &CompetitionHandler{svc: svc}
}

func (h *CompetitionHandler) List(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.ListActive(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cs == nil {
		cs = []model.Competition{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"competitions": cs})
}

func (h *CompetitionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	c, err := h.svc.Get(r.Context(), id)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *CompetitionHandler) ListChallenges(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	cs, err := h.svc.ListChallenges(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cs == nil {
		cs = []model.Challenge{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"challenges": cs})
}

type compCreateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
}

func (h *CompetitionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req compCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time format")
		return
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_time format")
		return
	}
	c := &model.Competition{
		Title:       req.Title,
		Description: req.Description,
		StartTime:   startTime,
		EndTime:     endTime,
	}
	id, err := h.svc.Create(r.Context(), c)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

type compUpdateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	IsActive    bool   `json:"is_active"`
}

func (h *CompetitionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	var req compUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	patch := &model.Competition{
		Title:       req.Title,
		Description: req.Description,
		IsActive:    req.IsActive,
	}
	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start_time format")
			return
		}
		patch.StartTime = t
	}
	if req.EndTime != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid end_time format")
			return
		}
		patch.EndTime = t
	}
	if err := h.svc.Update(r.Context(), id, patch); err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CompetitionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CompetitionHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cs == nil {
		cs = []model.Competition{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"competitions": cs})
}

func (h *CompetitionHandler) AddChallenge(w http.ResponseWriter, r *http.Request) {
	compID := parseID(r)
	var body struct {
		ChallengeID int64 `json:"challenge_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ChallengeID == 0 {
		writeError(w, http.StatusBadRequest, "challenge_id is required")
		return
	}
	if err := h.svc.AddChallenge(r.Context(), compID, body.ChallengeID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"competition_id": compID, "challenge_id": body.ChallengeID})
}

func (h *CompetitionHandler) RemoveChallenge(w http.ResponseWriter, r *http.Request) {
	compID := parseID(r)
	chalID, _ := strconv.ParseInt(chi.URLParam(r, "challenge_id"), 10, 64)
	if err := h.svc.RemoveChallenge(r.Context(), compID, chalID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
