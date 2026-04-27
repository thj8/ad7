// Package handler 实现比赛相关的 HTTP 请求处理。
// CompetitionHandler 负责比赛的 CRUD、题目分配、队伍管理等 HTTP 接口。
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"ad7/internal/logger"
	"ad7/internal/middleware"
	"ad7/internal/model"
	"ad7/internal/service"
)

// CompetitionHandler 处理比赛相关的 HTTP 请求。
// 持有 CompetitionService 用于业务逻辑调用，TeamResolver 用于检查访问权限。
type CompetitionHandler struct {
	svc          *service.CompetitionService
	teamResolver *service.TeamResolver
}

// NewCompetitionHandler 创建 CompetitionHandler 实例。
// 参数 svc: 比赛业务逻辑服务；tr: 队伍解析器。
func NewCompetitionHandler(svc *service.CompetitionService, tr *service.TeamResolver) *CompetitionHandler {
	return &CompetitionHandler{svc: svc, teamResolver: tr}
}

// List 处理 GET /api/v1/competitions 请求。
// 返回所有已激活比赛的列表（普通用户使用）。
func (h *CompetitionHandler) List(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.ListActive(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 确保空列表返回 [] 而非 null
	if cs == nil {
		cs = []model.Competition{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"competitions": cs})
}

// Get 处理 GET /api/v1/competitions/{id} 请求。
// 根据 res_id 获取单个比赛的详情。
func (h *CompetitionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := middleware.ID(r)
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

// ListChallenges 处理 GET /api/v1/competitions/{id}/challenges 请求。
// 返回指定比赛中所有已启用的题目列表。
// 队伍模式下会检查访问权限。
func (h *CompetitionHandler) ListChallenges(w http.ResponseWriter, r *http.Request) {
	id := middleware.ID(r)

	if err := h.svc.CheckCompAccess(r.Context(), id, middleware.UserID(r), h.teamResolver); err != nil {
		if err == service.ErrMustJoinTeam {
			writeError(w, http.StatusForbidden, "must join a team to participate")
			return
		}
		if err == service.ErrTeamNotRegistered {
			writeError(w, http.StatusForbidden, "your team is not registered for this competition")
			return
		}
		if err == service.ErrNotFound {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		logger.Error("check competition access", "error", err, "user_id", middleware.UserID(r))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

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

// compCreateRequest 是创建比赛的请求体结构。
// 时间字段使用字符串传输，在 Handler 中解析为 time.Time。
type compCreateRequest struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	Mode         string `json:"mode"`
	TeamJoinMode string `json:"team_join_mode"`
}

// Create 处理 POST /api/v1/admin/competitions 请求（管理员）。
// 解析请求体中的时间字符串（RFC3339 格式），创建新比赛。
// 返回 201 和新比赛的 res_id。
func (h *CompetitionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req compCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// 验证字段长度限制
	if validateLen("title", req.Title, maxTitleLen) != nil ||
		validateLen("description", req.Description, maxFieldLen) != nil {
		writeError(w, http.StatusBadRequest, "field too long")
		return
	}
	// 解析开始时间（RFC3339 格式）
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time format")
		return
	}
	// 解析结束时间（RFC3339 格式）
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_time format")
		return
	}
	c := &model.Competition{
		Title:         req.Title,
		Description:   req.Description,
		StartTime:     startTime,
		EndTime:       endTime,
		Mode:         model.CompetitionMode(req.Mode),
		TeamJoinMode: model.TeamJoinMode(req.TeamJoinMode),
	}
	id, err := h.svc.Create(r.Context(), c)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("competition created", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id, "title", req.Title)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// compUpdateRequest 是更新比赛的请求体结构。
type compUpdateRequest struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	IsActive      bool   `json:"is_active"`
	Mode         string `json:"mode"`
	TeamJoinMode string `json:"team_join_mode"`
}

// Update 处理 PUT /api/v1/admin/competitions/{id} 请求（管理员）。
// 使用合并策略更新比赛。时间字段只在非空时更新。
func (h *CompetitionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := middleware.ID(r)
	var req compUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// 验证字段长度限制
	if validateLen("title", req.Title, maxTitleLen) != nil ||
		validateLen("description", req.Description, maxFieldLen) != nil {
		writeError(w, http.StatusBadRequest, "field too long")
		return
	}
	patch := &model.Competition{
		Title:         req.Title,
		Description:   req.Description,
		IsActive:      req.IsActive,
		Mode:         model.CompetitionMode(req.Mode),
		TeamJoinMode: model.TeamJoinMode(req.TeamJoinMode),
	}
	// 仅在提供了开始时间时才更新
	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start_time format")
			return
		}
		patch.StartTime = t
	}
	// 仅在提供了结束时间时才更新
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
	} else if err == service.ErrInvalidMode {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("competition updated", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// Delete 处理 DELETE /api/v1/admin/competitions/{id} 请求（管理员）。
// 软删除指定比赛。
func (h *CompetitionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := middleware.ID(r)
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("competition deleted", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// ListAll 处理 GET /api/v1/admin/competitions 请求（管理员）。
// 返回所有比赛（含未激活的），与管理员路由挂载。
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

// AddChallenge 处理 POST /api/v1/admin/competitions/{id}/challenges 请求（管理员）。
// 将一道题目分配到指定比赛中。请求体需包含 challenge_id。
func (h *CompetitionHandler) AddChallenge(w http.ResponseWriter, r *http.Request) {
	compID := middleware.ID(r)
	var body struct {
		ChallengeID string `json:"challenge_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ChallengeID == "" {
		writeError(w, http.StatusBadRequest, "challenge_id is required")
		return
	}
	if err := h.svc.AddChallenge(r.Context(), compID, body.ChallengeID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("challenge assigned", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", compID, "challenge_id", body.ChallengeID)
	writeJSON(w, http.StatusCreated, map[string]any{"competition_id": compID, "challenge_id": body.ChallengeID})
}

// RemoveChallenge 处理 DELETE /api/v1/admin/competitions/{id}/challenges/{challenge_id} 请求（管理员）。
// 从指定比赛中移除一道题目。题目和比赛的 ID 均从 URL 路径参数获取。
func (h *CompetitionHandler) RemoveChallenge(w http.ResponseWriter, r *http.Request) {
	compID := middleware.ID(r)
	chalID := middleware.ChalID(r)
	if err := h.svc.RemoveChallenge(r.Context(), compID, chalID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("challenge removed", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", compID, "challenge_id", chalID)
	w.WriteHeader(http.StatusNoContent)
}

// compTeamRequest 是管理比赛队伍的请求体结构。
type compTeamRequest struct {
	TeamID string `json:"team_id"`
}

// compTeamResponse 是比赛队伍列表的响应结构。
type compTeamResponse struct {
	Teams []struct {
		ID string `json:"id"`
	} `json:"teams"`
}

// ListTeams 处理 GET /api/v1/competitions/{id}/teams 请求。
// 返回比赛中的队伍列表（仅管理员模式比赛使用）。
func (h *CompetitionHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	compID := middleware.ID(r)
	teams, err := h.svc.ListCompTeams(r.Context(), compID)
	if err != nil {
		logger.Error("list competition teams", "error", err, "user_id", middleware.UserID(r))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := compTeamResponse{
		Teams: make([]struct {
			ID string `json:"id"`
		}, len(teams)),
	}
	for i, t := range teams {
		resp.Teams[i].ID = t.TeamID
	}
	writeJSON(w, http.StatusOK, resp)
}

// AddTeam 处理 POST /api/v1/admin/competitions/{id}/teams 请求（管理员）。
// 将一支队伍添加到比赛中（仅管理员模式比赛使用）。
func (h *CompetitionHandler) AddTeam(w http.ResponseWriter, r *http.Request) {
	compID := middleware.ID(r)
	var req compTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	defer r.Body.Close()
	if err := validateLen("team_id", req.TeamID, 32); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.AddCompTeam(r.Context(), compID, req.TeamID); err != nil {
		if err == service.ErrCompNotTeamMode || err == service.ErrCompFreeMode {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err == service.ErrNotFound {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		logger.Error("add competition team", "error", err, "user_id", middleware.UserID(r))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// RemoveTeam 处理 DELETE /api/v1/admin/competitions/{id}/teams/{team_id} 请求（管理员）。
// 从比赛中移除一支队伍。
func (h *CompetitionHandler) RemoveTeam(w http.ResponseWriter, r *http.Request) {
	compID := middleware.ID(r)
	teamID := middleware.TeamID(r)

	if err := h.svc.RemoveCompTeam(r.Context(), compID, teamID); err != nil {
		if err == service.ErrCompNotTeamMode || err == service.ErrCompFreeMode {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err == service.ErrNotFound {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		logger.Error("remove competition team", "error", err, "user_id", middleware.UserID(r))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// Start 处理 POST /api/v1/admin/competitions/{id}/start 请求（管理员）。
// 手动激活指定比赛。返回更新后的比赛信息。
// 如果比赛已激活返回 409，不存在返回 404。
func (h *CompetitionHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := middleware.ID(r)
	c, err := h.svc.StartCompetition(r.Context(), id)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err == service.ErrConflict {
		writeError(w, http.StatusConflict, "competition already started")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("competition started", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id)
	writeJSON(w, http.StatusOK, c)
}

// End 处理 POST /api/v1/admin/competitions/{id}/end 请求（管理员）。
// 手动结束指定比赛。返回更新后的比赛信息。
// 如果比赛已结束返回 409，不存在返回 404。
func (h *CompetitionHandler) End(w http.ResponseWriter, r *http.Request) {
	id := middleware.ID(r)
	c, err := h.svc.EndCompetition(r.Context(), id)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err == service.ErrConflict {
		writeError(w, http.StatusConflict, "competition already ended")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("competition ended", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id)
	writeJSON(w, http.StatusOK, c)
}
