// Package handler 实现 HTTP 请求处理层（题目相关）。
// ChallengeHandler 负责题目的 CRUD HTTP 接口，使用单独的请求结构体
// 接收 Flag 字段（因为 model.Challenge.Flag 有 json:"-" 标签）。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"ad7/internal/ctxutil"
	"ad7/internal/logger"
	"ad7/internal/middleware"
	"ad7/internal/model"
	"ad7/internal/service"
)

// ChallengeHandler 处理题目相关的 HTTP 请求。
// 持有 ChallengeService 用于业务逻辑调用。
type ChallengeHandler struct {
	svc *service.ChallengeService
}

// NewChallengeHandler 创建 ChallengeHandler 实例。
// 参数 svc: 题目业务逻辑服务。
func NewChallengeHandler(svc *service.ChallengeService) *ChallengeHandler {
	return &ChallengeHandler{svc: svc}
}

// List 处理 GET /api/v1/challenges 请求。
// 返回所有已启用题目的列表（不含 Flag）。
func (h *ChallengeHandler) List(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 确保空列表返回 [] 而非 null
	if cs == nil {
		cs = []model.Challenge{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"challenges": cs})
}

// Get 处理 GET /api/v1/challenges/{id} 请求。
// 根据 URL 参数中的 res_id 获取单个题目详情（已通过中间件验证）。
// 返回 404 如果题目不存在。
func (h *ChallengeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := ctxutil.ID(r)
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

// createRequest 是创建题目的请求体结构。
// 与 model.Challenge 分开，因为 Challenge.Flag 有 json:"-" 标签，
// 无法直接从请求 JSON 反序列化 Flag 字段。
type createRequest struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Score       int    `json:"score"`
	Flag        string `json:"flag"`
}

// Create 处理 POST /api/v1/admin/challenges 请求（管理员）。
// 从请求体解析题目信息，验证字段长度，创建新题目。
// 返回 201 和新题目的 res_id。
func (h *ChallengeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// 手动将请求体字段赋值给 model（因为 Flag 不能自动反序列化）
	c := &model.Challenge{
		Title:       req.Title,
		Category:    req.Category,
		Description: req.Description,
		Score:       req.Score,
		Flag:        req.Flag,
	}
	id, err := h.svc.Create(r.Context(), c)
	if err != nil {
		var valErr *model.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusBadRequest, valErr.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("challenge created", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "challenge_id", id, "title", req.Title)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// updateRequest 是更新题目的请求体结构。
// 与 createRequest 类似，但多一个 is_enabled 字段用于控制题目启用状态。
type updateRequest struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Score       int    `json:"score"`
	Flag        string `json:"flag"`
	IsEnabled   bool   `json:"is_enabled"`
}

// Update 处理 PUT /api/v1/admin/challenges/{id} 请求（管理员）。
// 使用合并策略更新题目：只修改请求中非空/非零的字段。
// is_enabled 总是被显式设置。
func (h *ChallengeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := ctxutil.ID(r)
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// 构建 patch 对象用于合并更新
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
		var valErr *model.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusBadRequest, valErr.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("challenge updated", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "challenge_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// Delete 处理 DELETE /api/v1/admin/challenges/{id} 请求（管理员）。
// 软删除指定题目。
func (h *ChallengeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := ctxutil.ID(r)
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.Info("challenge deleted", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "challenge_id", id)
	w.WriteHeader(http.StatusNoContent)
}
