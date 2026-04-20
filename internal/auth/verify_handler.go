package auth

import (
	"net/http"
	"strings"
)

// VerifyHandler 处理 token 验证请求。
type VerifyHandler struct {
	svc *AuthService
}

// NewVerifyHandler 创建 VerifyHandler 实例。
func NewVerifyHandler(svc *AuthService) *VerifyHandler {
	return &VerifyHandler{svc: svc}
}

// Verify 处理 POST /api/v1/verify 请求。
// 从 Authorization 头提取 Bearer token，验证后返回 {user_id, role}。
func (h *VerifyHandler) Verify(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		authWriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
		return
	}
	tokenStr := strings.TrimPrefix(header, "Bearer ")

	userID, role, err := h.svc.VerifyToken(tokenStr)
	if err != nil {
		authWriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	authWriteJSON(w, http.StatusOK, map[string]string{
		"user_id": userID,
		"role":    role,
	})
}
