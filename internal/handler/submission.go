package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	challengeID, _ := strconv.Atoi(chi.URLParam(r, "id"))
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

func (h *SubmissionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	challengeID, _ := strconv.Atoi(r.URL.Query().Get("challenge_id"))
	subs, err := h.svc.List(r.Context(), userID, challengeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"submissions": subs})
}
