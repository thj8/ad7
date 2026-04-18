// Package hints 实现题目提示插件。
// 管理员可以为每道题目创建、更新、删除提示（支持软删除和可见性控制），
// 普通用户只能查看可见的提示列表。
package hints

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/uuid"
)

// Plugin 是提示插件，持有数据库连接。
type Plugin struct{ db *sql.DB }

// New 创建提示插件实例。
func New() *Plugin { return &Plugin{} }

// hint 表示一条题目提示。
type hint struct {
	ResID     string    `json:"id"`         // 提示的 UUID 标识
	Content   string    `json:"content"`    // 提示内容
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// createReq 是创建提示的请求体结构。
type createReq struct {
	Content string `json:"content"` // 提示内容（必填，最大 4096 字符）
}

// updateReq 是更新提示的请求体结构。
// 使用指针类型区分"未提供"和"设为空值"。
type updateReq struct {
	Content   *string `json:"content"`    // 提示内容（可选）
	IsVisible *bool   `json:"is_visible"` // 是否可见（可选）
}

// Register 注册提示相关的路由。
// 管理员路由：
//   - POST /api/v1/admin/challenges/{id}/hints（创建提示）
//   - PUT /api/v1/admin/hints/{id}（更新提示）
//   - DELETE /api/v1/admin/hints/{id}（删除提示）
//
// 用户路由：
//   - GET /api/v1/challenges/{id}/hints（查看可见提示）
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate, auth.RequireAdmin).Post("/api/v1/admin/challenges/{id}/hints", p.create)
	r.With(auth.Authenticate, auth.RequireAdmin).Put("/api/v1/admin/hints/{id}", p.update)
	r.With(auth.Authenticate, auth.RequireAdmin).Delete("/api/v1/admin/hints/{id}", p.delete)
	r.With(auth.Authenticate).Get("/api/v1/challenges/{id}/hints", p.list)
}

// create 处理创建提示的请求（管理员）。
// 验证题目 ID 格式和提示内容，插入数据库。
func (p *Plugin) create(w http.ResponseWriter, r *http.Request) {
	chalID := chi.URLParam(r, "id")
	// 验证题目 ID 格式（32 字符 UUID）
	if len(chalID) != 32 {
		http.Error(w, `{"error":"invalid challenge id"}`, http.StatusBadRequest)
		return
	}

	var req createReq
	// 验证内容非空且不超过 4096 字符
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" || len(req.Content) > 4096 {
		http.Error(w, `{"error":"content is required (max 4096 chars)"}`, http.StatusBadRequest)
		return
	}

	// 插入提示记录，默认不可见
	_, err := p.db.ExecContext(r.Context(),
		`INSERT INTO hints (res_id, challenge_id, content) VALUES (?, ?, ?)`,
		uuid.Next(), chalID, req.Content)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// update 处理更新提示的请求（管理员）。
// 使用合并策略：只更新请求中提供的字段，未提供的保持不变。
// 支持更新 content 和 is_visible 两个字段。
func (p *Plugin) update(w http.ResponseWriter, r *http.Request) {
	hintID := chi.URLParam(r, "id")
	if len(hintID) != 32 {
		http.Error(w, `{"error":"invalid hint id"}`, http.StatusBadRequest)
		return
	}

	var req updateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// 验证内容长度（如果提供了的话）
	if req.Content != nil && (len(*req.Content) == 0 || len(*req.Content) > 4096) {
		http.Error(w, `{"error":"content must be 1-4096 chars"}`, http.StatusBadRequest)
		return
	}

	// 获取当前值用于合并
	var currentContent string
	var currentIsVisible bool
	err := p.db.QueryRowContext(r.Context(),
		`SELECT content, is_visible FROM hints WHERE res_id = ? AND is_deleted = 0`, hintID).
		Scan(&currentContent, &currentIsVisible)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	// 确定新值：未提供的字段保持原值
	newContent := currentContent
	if req.Content != nil {
		newContent = *req.Content
	}
	newIsVisible := currentIsVisible
	if req.IsVisible != nil {
		newIsVisible = *req.IsVisible
	}

	// 执行更新
	_, err = p.db.ExecContext(r.Context(),
		`UPDATE hints SET content = ?, is_visible = ? WHERE res_id = ? AND is_deleted = 0`,
		newContent, newIsVisible, hintID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// delete 处理删除提示的请求（管理员）。
// 使用软删除，将 is_deleted 设为 1。
// 如果提示不存在返回 404。
func (p *Plugin) delete(w http.ResponseWriter, r *http.Request) {
	hintID := chi.URLParam(r, "id")
	if len(hintID) != 32 {
		http.Error(w, `{"error":"invalid hint id"}`, http.StatusBadRequest)
		return
	}

	// 软删除提示
	result, err := p.db.ExecContext(r.Context(),
		`UPDATE hints SET is_deleted = 1 WHERE res_id = ? AND is_deleted = 0`, hintID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	// 检查是否有记录被删除
	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// list 处理获取题目可见提示列表的请求。
// 只返回 is_visible = 1 且未删除的提示，按创建时间升序排列。
func (p *Plugin) list(w http.ResponseWriter, r *http.Request) {
	chalID := chi.URLParam(r, "id")
	if len(chalID) != 32 {
		http.Error(w, `{"error":"invalid challenge id"}`, http.StatusBadRequest)
		return
	}

	// 查询可见且未删除的提示
	rows, err := p.db.QueryContext(r.Context(),
		`SELECT res_id, content, created_at FROM hints
		 WHERE challenge_id = ? AND is_visible = 1 AND is_deleted = 0
		 ORDER BY created_at ASC`, chalID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hints []hint
	for rows.Next() {
		var h hint
		if err := rows.Scan(&h.ResID, &h.Content, &h.CreatedAt); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		hints = append(hints, h)
	}

	// 确保空列表返回 [] 而非 null
	if hints == nil {
		hints = []hint{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"hints": hints})
}
