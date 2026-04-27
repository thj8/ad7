// Package handler 实现 Flag 提交相关的 HTTP 请求处理。
// SubmissionHandler 负责比赛内的 Flag 提交和提交记录查询等接口。
package handler

import (
	"encoding/json"
	"net/http"

	"ad7/internal/middleware"
	"ad7/internal/service"
	"ad7/internal/store"
)

// SubmissionHandler 处理 Flag 提交相关的 HTTP 请求。
// 持有 SubmissionService 用于业务逻辑调用。
type SubmissionHandler struct {
	svc *service.SubmissionService
}

// NewSubmissionHandler 创建 SubmissionHandler 实例。
func NewSubmissionHandler(svc *service.SubmissionService) *SubmissionHandler {
	return &SubmissionHandler{svc: svc}
}

// SubmitInComp 处理 POST /api/v1/competitions/{comp_id}/challenges/{id}/submit 请求。
// 比赛范围内的 Flag 提交。从 Context 获取已验证的比赛 ID 和题目 ID。
// 返回提交结果：correct / incorrect / already_solved。
func (h *SubmissionHandler) SubmitInComp(w http.ResponseWriter, r *http.Request) {
	compID := middleware.CompID(r)
	chalID := middleware.ChalID(r)

	var body struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Flag == "" {
		writeError(w, http.StatusBadRequest, "flag is required")
		return
	}
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
	if err == service.ErrCompetitionNotActive {
		writeError(w, http.StatusBadRequest, "competition is not active")
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

// ListByComp 处理 GET /api/v1/admin/competitions/{id}/submissions 请求（管理员）。
// 支持通过 query 参数 user_id 和 challenge_id 过滤提交记录。
func (h *SubmissionHandler) ListByComp(w http.ResponseWriter, r *http.Request) {
	compID := middleware.ID(r)
	userID := r.URL.Query().Get("user_id")
	challengeID := r.URL.Query().Get("challenge_id")
	subs, err := h.svc.ListByComp(r.Context(), store.ListSubmissionsParams{
		CompetitionID: compID,
		UserID:        userID,
		ChallengeID:   challengeID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"submissions": subs})
}
