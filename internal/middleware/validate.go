// Package middleware 提供 HTTP 中间件。
package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ad7/internal/uuid"
)

// ctxKey 是用于存储 URL 参数的 Context 键类型。
type ctxKey string

// Context 键常量（用于中间件和路由使用）。
const (
	CtxKeyCompID ctxKey = "comp_id"
	CtxKeyChalID ctxKey = "chal_id"
	CtxKeyTeamID ctxKey = "team_id"
	CtxKeyID     ctxKey = "id" // 通用 ID（单个参数的路由）
)

// ValidateURLParam 验证 URL 参数并将其存入 Context。
// 参数名必须与 chi.URLParam 使用的名称一致。
// 验证通过后，可通过对应的 Getter 函数（如 CompID）从 Context 中获取。
func ValidateURLParam(paramName string, ctxKey ctxKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, paramName)
			if err := uuid.Validate(id); err != nil {
				writeError(w, http.StatusBadRequest, "invalid "+paramName)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CompID 从 Context 中获取比赛 ID。
// 必须在使用 ValidateURLParam("id", CtxKeyCompID) 或
// ValidateURLParam("comp_id", CtxKeyCompID) 的路由中调用。
func CompID(r *http.Request) string {
	return r.Context().Value(CtxKeyCompID).(string)
}

// ChalID 从 Context 中获取题目 ID。
// 必须在使用 ValidateURLParam("id", CtxKeyChalID) 或
// ValidateURLParam("challenge_id", CtxKeyChalID) 的路由中调用。
func ChalID(r *http.Request) string {
	return r.Context().Value(CtxKeyChalID).(string)
}

// TeamID 从 Context 中获取队伍 ID。
// 必须在使用 ValidateURLParam("team_id", CtxKeyTeamID) 的路由中调用。
func TeamID(r *http.Request) string {
	return r.Context().Value(CtxKeyTeamID).(string)
}

// ID 从 Context 中获取通用 ID（单参数路由）。
// 必须在使用 ValidateURLParam("id", CtxKeyID) 的路由中调用。
func ID(r *http.Request) string {
	return r.Context().Value(CtxKeyID).(string)
}

// writeError 是 handler 包 writeError 的简化版，避免循环导入。
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": msg,
	})
}
