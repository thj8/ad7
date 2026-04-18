// Package notification 实现比赛通知插件。
// 管理员可以为每个比赛创建通知，所有用户可以查看比赛下的通知列表。
package notification

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/model"
	"ad7/internal/uuid"
)

// Plugin 是通知插件，持有数据库连接。
type Plugin struct{ db *sql.DB }

// New 创建通知插件实例。
func New() *Plugin { return &Plugin{} }

// Register 注册通知相关的路由。
// 路由：
//   - POST /api/v1/admin/competitions/{id}/notifications（管理员，创建通知）
//   - GET /api/v1/competitions/{id}/notifications（认证用户，查看通知列表）
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/competitions/{id}/notifications", p.createForComp)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/notifications", p.listByComp)
}

// createReq 是创建通知的请求体结构。
type createReq struct {
	Title   string `json:"title"`   // 通知标题
	Message string `json:"message"` // 通知内容
}

// listByComp 处理获取比赛通知列表的请求。
// 查询指定比赛的所有通知，按创建时间倒序排列。
func (p *Plugin) listByComp(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	rows, err := p.db.QueryContext(r.Context(), `
		SELECT res_id, competition_id, title, message, created_at
		FROM notifications
		WHERE competition_id = ? AND is_deleted = 0
		ORDER BY created_at DESC`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ns []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ResID, &n.CompetitionID, &n.Title, &n.Message, &n.CreatedAt); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		ns = append(ns, n)
	}
	// 确保空列表返回 [] 而非 null
	if ns == nil {
		ns = []model.Notification{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"notifications": ns})
}

// createForComp 处理创建比赛通知的请求（管理员）。
// 验证 title 和 message 均为必填，插入数据库后返回 201。
func (p *Plugin) createForComp(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	var req createReq
	// 验证请求体和必填字段
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.Message == "" {
		http.Error(w, `{"error":"title and message are required"}`, http.StatusBadRequest)
		return
	}
	// 插入通知记录，自动生成 res_id
	_, err := p.db.ExecContext(r.Context(),
		`INSERT INTO notifications (res_id, competition_id, title, message) VALUES (?, ?, ?, ?)`,
		uuid.Next(), compID, req.Title, req.Message)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
