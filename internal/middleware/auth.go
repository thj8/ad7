// Package middleware 提供 HTTP 中间件，包括 JWT 认证和管理员权限校验。
// Authenticate 中间件从请求头提取并验证 JWT Token，将用户信息注入请求上下文。
// RequireAdmin 中间件检查用户角色是否为管理员，非管理员返回 403。
package middleware

import (
	"context"
	"net/http"
	"strings"

	"ad7/internal/logger"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey 是上下文键的类型，避免与其它包的键冲突。
type contextKey string

const (
	// CtxUserID 是存储在上下文中的用户 ID 键（来自 JWT 的 sub claim）
	CtxUserID contextKey = "user_id"
	// CtxRole 是存储在上下文中的用户角色键（来自 JWT 的 role claim）
	CtxRole   contextKey = "role"
)

// Auth 封装 JWT 认证相关的配置。
type Auth struct {
	secret    []byte // JWT 签名密钥
	adminRole string // 管理员角色名称，用于权限校验
}

// NewAuth 创建 Auth 中间件实例。
// 参数：
//   - secret: JWT 签名密钥字符串
//   - adminRole: 管理员角色名称（如 "admin"）
func NewAuth(secret, adminRole string) *Auth {
	return &Auth{secret: []byte(secret), adminRole: adminRole}
}

// Authenticate 是 JWT 认证中间件。
// 从请求头 Authorization: Bearer <token> 提取 JWT Token，
// 验证签名和有效性，提取 sub（用户 ID）和 role（角色）注入请求上下文。
// 如果 Token 缺失或无效，返回 401 Unauthorized。
func (a *Auth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 提取 Authorization 头
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			logger.Warn("auth failed", "error", "missing token")
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		// 解析并验证 JWT Token，确认使用 HMAC 签名算法
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return a.secret, nil
		})
		if err != nil || !token.Valid {
			logger.Warn("auth failed", "error", "invalid token")
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		// 从 claims 中提取用户 ID 和角色
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			logger.Warn("auth failed", "error", "invalid claims")
			http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
			return
		}
		userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		http.Error(w, `{"error":"invalid token: missing sub"}`, http.StatusUnauthorized)
		return
	}
		role, ok := claims["role"].(string)
	if !ok || role == "" {
		http.Error(w, `{"error":"invalid token: missing role"}`, http.StatusUnauthorized)
		return
	}
		// 将用户信息注入请求上下文，供后续 Handler 使用
		ctx := context.WithValue(r.Context(), CtxUserID, userID)
		ctx = context.WithValue(ctx, CtxRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin 是管理员权限校验中间件。
// 检查请求上下文中的角色是否匹配配置的管理员角色名。
// 非管理员返回 403 Forbidden。必须放在 Authenticate 中间件之后使用。
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
// 这是一个便捷函数，供 Handler 层获取当前认证用户的 ID。
func UserID(r *http.Request) string {
	v, _ := r.Context().Value(CtxUserID).(string)
	return v
}
