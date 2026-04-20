// Package notification 实现比赛通知插件。
// 管理员可以为每个比赛创建通知，所有用户可以查看比赛下的通知列表。
package notification

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ad7/internal/logger"
	"ad7/internal/middleware"
	"ad7/internal/model"
	"ad7/internal/plugin"
	"ad7/internal/pluginutil"
	"ad7/internal/uuid"
)

// Plugin 是通知插件，持有数据库连接。
type Plugin struct{ db *sql.DB }

// New 创建通知插件实例。
func New() *Plugin { return &Plugin{} }

// Name 返回插件名称
func (p *Plugin) Name() string {
	return plugin.NameNotification
}

// updateReq 是更新通知的请求体结构。
// 使用指针类型区分"未提供"和"设为空值"。
type updateReq struct {
	Title   *string `json:"title"`   // 通知标题（可选）
	Message *string `json:"message"` // 通知内容（可选）
}

// Register 注册通知相关的路由。
// 管理员路由：
//   - POST /api/v1/admin/competitions/{id}/notifications（创建通知）
//   - PUT /api/v1/admin/notifications/{id}（更新通知）
//   - DELETE /api/v1/admin/notifications/{id}（删除通知）
//
// 用户路由：
//   - GET /api/v1/competitions/{id}/notifications（查看通知列表）
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/competitions/{id}/notifications", p.createForComp)
	r.With(auth.Authenticate, auth.RequireAdmin).Put("/api/v1/admin/notifications/{id}", p.update)
	r.With(auth.Authenticate, auth.RequireAdmin).Delete("/api/v1/admin/notifications/{id}", p.delete)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/notifications", p.listByComp)
}

// createReq 是创建通知的请求体结构。
type createReq struct {
	Title   string `json:"title"`   // 通知标题
	Message string `json:"message"` // 通知内容
}

// listByComp 处理获取比赛通知列表的请求。
func (p *Plugin) listByComp(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}
	rows, err := p.db.QueryContext(r.Context(), `
		SELECT res_id, competition_id, title, message, created_at, updated_at
		FROM notifications
		WHERE competition_id = ? AND is_deleted = 0
		ORDER BY created_at DESC`, compID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	defer rows.Close()

	var ns []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ResID, &n.CompetitionID, &n.Title, &n.Message, &n.CreatedAt, &n.UpdatedAt); err != nil {
			pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
			return
		}
		ns = append(ns, n)
	}
	if ns == nil {
		ns = []model.Notification{}
	}
	pluginutil.WriteJSON(w, http.StatusOK, map[string]any{"notifications": ns})
}

// createForComp 处理创建比赛通知的请求（管理员）。
func (p *Plugin) createForComp(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.Message == "" {
		pluginutil.WriteError(w, http.StatusBadRequest, "title and message are required")
		return
	}
	if len(req.Title) > 255 || len(req.Message) > 4096 {
		pluginutil.WriteError(w, http.StatusBadRequest, "title too long (max 255) or message too long (max 4096)")
		return
	}
	_, err := p.db.ExecContext(r.Context(),
		`INSERT INTO notifications (res_id, competition_id, title, message) VALUES (?, ?, ?, ?)`,
		uuid.Next(), compID, req.Title, req.Message)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	logger.Info("notification created", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "competition_id", compID)
	w.WriteHeader(http.StatusCreated)
}

// update 处理更新通知的请求（管理员）。
func (p *Plugin) update(w http.ResponseWriter, r *http.Request) {
	notifID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(notifID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	var req updateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Title != nil && (len(*req.Title) == 0 || len(*req.Title) > 255) {
		pluginutil.WriteError(w, http.StatusBadRequest, "title must be 1-255 chars")
		return
	}
	if req.Message != nil && (len(*req.Message) == 0 || len(*req.Message) > 4096) {
		pluginutil.WriteError(w, http.StatusBadRequest, "message must be 1-4096 chars")
		return
	}

	var currentTitle string
	var currentMessage string
	err := p.db.QueryRowContext(r.Context(),
		`SELECT title, message FROM notifications WHERE res_id = ? AND is_deleted = 0`, notifID).
		Scan(&currentTitle, &currentMessage)
	if err == sql.ErrNoRows {
		pluginutil.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	newTitle := currentTitle
	if req.Title != nil {
		newTitle = *req.Title
	}
	newMessage := currentMessage
	if req.Message != nil {
		newMessage = *req.Message
	}

	_, err = p.db.ExecContext(r.Context(),
		`UPDATE notifications SET title = ?, message = ? WHERE res_id = ? AND is_deleted = 0`,
		newTitle, newMessage, notifID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	logger.Info("notification updated", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "notification_id", notifID)
	w.WriteHeader(http.StatusNoContent)
}

// delete 处理删除通知的请求（管理员）。
func (p *Plugin) delete(w http.ResponseWriter, r *http.Request) {
	notifID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(notifID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	result, err := p.db.ExecContext(r.Context(),
		`UPDATE notifications SET is_deleted = 1, updated_at = NOW() WHERE res_id = ? AND is_deleted = 0`, notifID)
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	if rows == 0 {
		pluginutil.WriteError(w, http.StatusNotFound, "not found")
		return
	}

	logger.Info("notification deleted", "user", middleware.UserID(r), "role", r.Context().Value(middleware.CtxRole), "notification_id", notifID)
	w.WriteHeader(http.StatusNoContent)
}