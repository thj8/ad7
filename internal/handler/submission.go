package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/service"
)

type SubmissionHandler struct {
	svc *service.SubmissionService
}

func NewSubmissionHandler(svc *service.SubmissionService) *SubmissionHandler {
	return &SubmissionHandler{svc: svc}
}

func (h *SubmissionHandler) Submit(w http.ResponseWriter, r *http.Request) {
	challengeID := chi.URLParam(r, "id")
	var body struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Flag == "" {
		writeError(w, http.StatusBadRequest, "flag is required")
		return
	}
	userID := middleware.UserID(r)
	result, err := h.svc.Submit(r.Context(), userID, challengeID, body.Flag)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "challenge not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": result == service.ResultCorrect,
		"message": string(result),
	})
}

func (h *SubmissionHandler) SubmitInComp(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "comp_id")
	chalID := chi.URLParam(r, "id")
	var body struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Flag == "" {
		writeError(w, http.StatusBadRequest, "flag is required")
		return
	}
	userID := middleware.UserID(r)
	result, err := h.svc.SubmitInComp(r.Context(), userID, compID, chalID, body.Flag)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "challenge not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": result == service.ResultCorrect,
		"message": string(result),
	})
}

func (h *SubmissionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	challengeID := r.URL.Query().Get("challenge_id")
	subs, err := h.svc.List(r.Context(), userID, challengeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"submissions": subs})
}
