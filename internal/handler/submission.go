// Package handler 实现 Flag 提交相关的 HTTP 请求处理。
// SubmissionHandler 负责全局和比赛内的 Flag 提交、提交记录查询等接口。
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/service"
)

// SubmissionHandler 处理 Flag 提交相关的 HTTP 请求。
// 持有 SubmissionService 用于业务逻辑调用。
type SubmissionHandler struct {
	svc *service.SubmissionService
}

// NewSubmissionHandler 创建 SubmissionHandler 实例。
// 参数 svc: 提交业务逻辑服务。
func NewSubmissionHandler(svc *service.SubmissionService) *SubmissionHandler {
	return &SubmissionHandler{svc: svc}
}

// Submit 处理 POST /api/v1/challenges/{id}/submit 请求。
// 全局范围内的 Flag 提交。从 JWT 中获取用户 ID，从 URL 获取题目 ID。
// 返回提交结果：correct / incorrect / already_solved。
func (h *SubmissionHandler) Submit(w http.ResponseWriter, r *http.Request) {
	// 从 URL 路径获取题目 res_id
	challengeID := chi.URLParam(r, "id")
	var body struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Flag == "" {
		writeError(w, http.StatusBadRequest, "flag is required")
		return
	}
	// 从 JWT 认证中间件注入的上下文中获取用户 ID
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

// SubmitInComp 处理 POST /api/v1/competitions/{comp_id}/challenges/{id}/submit 请求。
// 比赛范围内的 Flag 提交。从 URL 获取比赛 ID 和题目 ID。
// 返回提交结果：correct / incorrect / already_solved。
func (h *SubmissionHandler) SubmitInComp(w http.ResponseWriter, r *http.Request) {
	// 从 URL 路径获取比赛 ID 和题目 ID
	compID := chi.URLParam(r, "comp_id")
	chalID := chi.URLParam(r, "id")
	var body struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Flag == "" {
		writeError(w, http.StatusBadRequest, "flag is required")
		return
	}
	// 从上下文获取用户 ID
	userID := middleware.UserID(r)
	result, err := h.svc.SubmitInComp(r.Context(), &service.SubmitInCompRequest{
		UserID:        userID,
		CompetitionID: compID,
		ChallengeID:   chalID,
		Flag:          body.Flag,
	})
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

// List 处理 GET /api/v1/admin/submissions 请求（管理员）。
// 支持通过 query 参数 user_id 和 challenge_id 过滤提交记录。
func (h *SubmissionHandler) List(w http.ResponseWriter, r *http.Request) {
	// 从 query 参数获取可选的过滤条件
	userID := r.URL.Query().Get("user_id")
	challengeID := r.URL.Query().Get("challenge_id")
	subs, err := h.svc.List(r.Context(), userID, challengeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"submissions": subs})
}
