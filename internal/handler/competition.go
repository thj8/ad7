// Package handler 实现比赛相关的 HTTP 请求处理。
// CompetitionHandler 负责比赛的 CRUD、题目分配等 HTTP 接口。
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/logger"
	"ad7/internal/middleware"
	"ad7/internal/model"
	"ad7/internal/service"
)

// CompetitionHandler 处理比赛相关的 HTTP 请求。
// 持有 CompetitionService 用于业务逻辑调用。
type CompetitionHandler struct {
	svc *service.CompetitionService
}

// NewCompetitionHandler 创建 CompetitionHandler 实例。
// 参数 svc: 比赛业务逻辑服务。
func NewCompetitionHandler(svc *service.CompetitionService) *CompetitionHandler {
	return &CompetitionHandler{svc: svc}
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
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
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
func (h *CompetitionHandler) ListChallenges(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
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
	Title       string `json:"title"`
	Description string `json:"description"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
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
	if validateLen("title", req.Title, 255) != "" ||
		validateLen("description", req.Description, maxFieldLen) != "" {
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
	logger.Info("competition created", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", id, "title", req.Title)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// compUpdateRequest 是更新比赛的请求体结构。
// 与创建请求的区别在于多一个 is_active 字段，且时间字段可选。
type compUpdateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	IsActive    bool   `json:"is_active"`
}

// Update 处理 PUT /api/v1/admin/competitions/{id} 请求（管理员）。
// 使用合并策略更新比赛。时间字段只在非空时更新。
func (h *CompetitionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req compUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// 验证字段长度限制
	if validateLen("title", req.Title, 255) != "" ||
		validateLen("description", req.Description, maxFieldLen) != "" {
		writeError(w, http.StatusBadRequest, "field too long")
		return
	}
	patch := &model.Competition{
		Title:       req.Title,
		Description: req.Description,
		IsActive:    req.IsActive,
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
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
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
	compID, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
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
	compID, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	// 从 URL 获取题目 ID（无需 32 字符验证，由数据库处理）
	chalID := chi.URLParam(r, "challenge_id")
	if chalID == "" {
		writeError(w, http.StatusBadRequest, "invalid challenge_id")
		return
	}
	if err := h.svc.RemoveChallenge(r.Context(), compID, chalID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("challenge removed", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", compID, "challenge_id", chalID)
	w.WriteHeader(http.StatusNoContent)
}

// Start 处理 POST /api/v1/admin/competitions/{id}/start 请求（管理员）。
// 手动激活指定比赛。返回更新后的比赛信息。
// 如果比赛已激活返回 409，不存在返回 404。
func (h *CompetitionHandler) Start(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
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
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
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
