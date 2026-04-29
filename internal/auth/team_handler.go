package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ad7/internal/basemodel"
	"ad7/internal/logger"
	"ad7/internal/middleware"
)

// TeamHandler 处理队伍管理的 HTTP 请求。
type TeamHandler struct {
	svc *TeamService
}

// NewTeamHandler 创建 TeamHandler 实例。
func NewTeamHandler(svc *TeamService) *TeamHandler {
	return &TeamHandler{svc: svc}
}

// List 处理 GET /api/v1/teams 请求。返回所有队伍列表。
func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	teams, err := h.svc.ListTeams(r.Context())
	if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if teams == nil {
		teams = []Team{}
	}
	authWriteJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

// Get 处理 GET /api/v1/teams/{id} 请求。返回单个队伍详情。
func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if len(id) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	t, err := h.svc.GetTeam(r.Context(), id)
	if err == ErrNotFound {
		authWriteError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	authWriteJSON(w, http.StatusOK, t)
}

// Create 处理 POST /api/v1/admin/teams 请求（管理员）。
func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		authWriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	t := &Team{
		Name:        body.Name,
		Description: body.Description,
	}
	id, err := h.svc.CreateTeam(r.Context(), t)
	if err != nil {
		var valErr *basemodel.ValidationError
		if errors.As(err, &valErr) {
			authWriteError(w, http.StatusBadRequest, valErr.Error())
			return
		}
		authWriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("team created", "user", middleware.UserID(r), "team_id", id, "name", body.Name)
	authWriteJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// Update 处理 PUT /api/v1/admin/teams/{id} 请求（管理员）。
func (h *TeamHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if len(id) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		authWriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	t := &Team{
		Name:        body.Name,
		Description: body.Description,
	}
	t.ResID = id
	if err := h.svc.UpdateTeam(r.Context(), t); err != nil {
		var valErr *basemodel.ValidationError
		if errors.As(err, &valErr) {
			authWriteError(w, http.StatusBadRequest, valErr.Error())
			return
		}
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("team updated", "user", middleware.UserID(r), "team_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// Delete 处理 DELETE /api/v1/admin/teams/{id} 请求（管理员）。
func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if len(id) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.DeleteTeam(r.Context(), id); err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("team deleted", "user", middleware.UserID(r), "team_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// AddMember 处理 POST /api/v1/admin/teams/{id}/members 请求（管理员）。
func (h *TeamHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	if len(teamID) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"` // 可选，默认为 "member"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
		authWriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if err := h.svc.AddMember(r.Context(), teamID, body.UserID, body.Role); err == ErrNotFound {
		authWriteError(w, http.StatusNotFound, err.Error())
		return
	} else if err == ErrUserAlreadyInTeam {
		authWriteError(w, http.StatusConflict, err.Error())
		return
	} else if err == ErrUserAlreadyMember {
		authWriteError(w, http.StatusConflict, err.Error())
		return
	} else if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("member added to team", "user", middleware.UserID(r), "team_id", teamID, "member_id", body.UserID)
	authWriteJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

// RemoveMember 处理 DELETE /api/v1/admin/teams/{id}/members/{user_id} 请求（管理员）。
func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	if len(teamID) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		authWriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.svc.RemoveMember(r.Context(), teamID, userID); err == ErrNotFound {
		authWriteError(w, http.StatusNotFound, err.Error())
		return
	} else if err == ErrUserNotMember {
		authWriteError(w, http.StatusBadRequest, err.Error())
		return
	} else if err == ErrCannotRemoveCaptain {
		authWriteError(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("member removed from team", "user", middleware.UserID(r), "team_id", teamID, "member_id", userID)
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers 处理 GET /api/v1/teams/{id}/members 请求。
func (h *TeamHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if len(id) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	members, err := h.svc.ListMembers(r.Context(), id)
	if err == ErrNotFound {
		authWriteError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if members == nil {
		members = []MemberInfo{}
	}
	authWriteJSON(w, http.StatusOK, map[string]any{"members": members})
}

// SetCaptain 处理 PUT /api/v1/admin/teams/{id}/captain 请求（管理员）。
func (h *TeamHandler) SetCaptain(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	if len(teamID) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
		authWriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if err := h.svc.SetCaptain(r.Context(), teamID, body.UserID); err == ErrNotFound {
		authWriteError(w, http.StatusNotFound, err.Error())
		return
	} else if err == ErrUserNotMember {
		authWriteError(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("captain set", "user", middleware.UserID(r), "team_id", teamID, "captain_id", body.UserID)
	authWriteJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

// GetUserTeams 处理 GET /api/v1/users/{userID}/teams 请求。
func (h *TeamHandler) GetUserTeams(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if len(userID) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	teams, err := h.svc.GetUserTeamsInfo(r.Context(), userID)
	if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if teams == nil {
		teams = []Team{}
	}

	authWriteJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

// TransferCaptain 处理 POST /api/v1/admin/teams/{id}/transfer-captain 请求（管理员）。
func (h *TeamHandler) TransferCaptain(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	if len(teamID) != 32 {
		authWriteError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var body struct {
		ToUserID string `json:"to_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ToUserID == "" {
		authWriteError(w, http.StatusBadRequest, "to_user_id is required")
		return
	}
	// 从 context 获取当前用户作为 fromUserID
	fromUserID := middleware.UserID(r)
	if fromUserID == "" {
		authWriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.TransferCaptain(r.Context(), teamID, fromUserID, body.ToUserID); err == ErrNotFound {
		authWriteError(w, http.StatusNotFound, err.Error())
		return
	} else if err == ErrUserNotMember {
		authWriteError(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("captain transferred", "user", middleware.UserID(r), "team_id", teamID, "to_user_id", body.ToUserID)
	authWriteJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}
