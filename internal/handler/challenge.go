package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ad7/internal/model"
	"ad7/internal/service"
)

type ChallengeHandler struct {
	svc *service.ChallengeService
}

func NewChallengeHandler(svc *service.ChallengeService) *ChallengeHandler {
	return &ChallengeHandler{svc: svc}
}

func (h *ChallengeHandler) List(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cs == nil {
		cs = []model.Challenge{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"challenges": cs})
}

func (h *ChallengeHandler) Get(w http.ResponseWriter, r *http.Request) {
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

type createRequest struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Score       int    `json:"score"`
	Flag        string `json:"flag"`
}

func (h *ChallengeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	c := &model.Challenge{
		Title:       req.Title,
		Category:    req.Category,
		Description: req.Description,
		Score:       req.Score,
		Flag:        req.Flag,
	}
	id, err := h.svc.Create(r.Context(), c)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

type updateRequest struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Score       int    `json:"score"`
	Flag        string `json:"flag"`
	IsEnabled   bool   `json:"is_enabled"`
}

func (h *ChallengeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	patch := &model.Challenge{
		Title:       req.Title,
		Category:    req.Category,
		Description: req.Description,
		Score:       req.Score,
		Flag:        req.Flag,
		IsEnabled:   req.IsEnabled,
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

func (h *ChallengeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseID(r *http.Request) int {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	return id
}
