// Package middleware 提供 HTTP 中间件，包括 JWT 认证和管理员权限校验。
// Authenticate 中间件调用独立认证服务的 /api/v1/verify 接口验证 token。
// RequireAdmin 中间件检查用户角色是否为管理员，非管理员返回 403。
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ad7/internal/logger"
)

// contextKey 是上下文键的类型，避免与其它包的键冲突。
type contextKey string

const (
	// CtxUserID 是存储在上下文中的用户 ID 键
	CtxUserID contextKey = "user_id"
	// CtxRole 是存储在上下文中的用户角色键
	CtxRole contextKey = "role"
)

// Auth 封装认证中间件配置。
type Auth struct {
	authURL   string       // 认证服务地址
	adminRole string       // 管理员角色名称
	client    *http.Client // HTTP 客户端（带超时）
}

// NewAuth 创建 Auth 中间件实例。
// 参数：
//   - authURL: 认证服务地址（如 "http://localhost:8081"）
//   - adminRole: 管理员角色名称（如 "admin"）
func NewAuth(authURL, adminRole string) *Auth {
	return &Auth{
		authURL:   authURL,
		adminRole: adminRole,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// verifyResponse 是 /verify 接口的响应结构。
type verifyResponse struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// Authenticate 是认证中间件。
// 从请求头提取 Bearer token，调用认证服务的 /api/v1/verify 接口验证，
// 将 user_id 和 role 注入请求上下文。
func (a *Auth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			logger.Warn("auth failed", "error", "missing token")
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}

		userID, role, err := a.verifyToken(r, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			logger.Warn("auth failed", "error", err.Error())
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), CtxUserID, userID)
		ctx = context.WithValue(ctx, CtxRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// verifyToken 调用认证服务验证 token。
func (a *Auth) verifyToken(r *http.Request, tokenStr string) (string, string, error) {
	req, err := http.NewRequestWithContext(r.Context(), "POST", a.authURL+"/api/v1/verify", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 丢弃响应体，避免连接复用问题
		io.Copy(io.Discard, resp.Body)
		return "", "", fmt.Errorf("verify returned status %d", resp.StatusCode)
	}

	var vr verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return "", "", err
	}
	return vr.UserID, vr.Role, nil
}

// RequireAdmin 是管理员权限校验中间件。
func (a *Auth) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(CtxRole).(string)
		userID, _ := r.Context().Value(CtxUserID).(string)
		if role != a.adminRole {
			logger.Warn("access denied", "user", userID, "role", role, "required_role", a.adminRole)
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserID 从请求上下文中提取用户 ID。
func UserID(r *http.Request) string {
	v, _ := r.Context().Value(CtxUserID).(string)
	return v
}
