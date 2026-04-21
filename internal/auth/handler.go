package auth

import (
	"encoding/json"
	"net/http"
)

// AuthHandler 处理用户注册和登录的 HTTP 请求。
type AuthHandler struct {
	svc *AuthService
}

// NewAuthHandler 创建 AuthHandler 实例。
func NewAuthHandler(svc *AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Register 处理 POST /api/v1/register 请求。
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		authWriteError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	user, err := h.svc.Register(r.Context(), &RegisterRequest{
		Username: body.Username,
		Password: body.Password,
		Role:     body.Role,
	})
	if err == ErrConflict {
		authWriteError(w, http.StatusConflict, "username already exists")
		return
	}
	if err != nil {
		authWriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	authWriteJSON(w, http.StatusCreated, map[string]any{
		"id":       user.ResID,
		"username": user.Username,
		"role":     user.Role,
	})
}

// Login 处理 POST /api/v1/login 请求。
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		authWriteError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	token, err := h.svc.Login(r.Context(), &LoginRequest{
		Username: body.Username,
		Password: body.Password,
	})
	if err == ErrUnauthorized {
		authWriteError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err != nil {
		authWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	authWriteJSON(w, http.StatusOK, map[string]any{
		"token": token,
	})
}

// authWriteJSON 写入 JSON 响应。
func authWriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// authWriteError 写入错误响应。
func authWriteError(w http.ResponseWriter, status int, msg string) {
	authWriteJSON(w, status, map[string]string{"error": msg})
}
