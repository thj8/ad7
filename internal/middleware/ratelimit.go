// Package middleware 提供 HTTP 中间件，包括频率限制。
package middleware

import (
	"net/http"
	"time"

	"ad7/internal/logger"

	"github.com/go-chi/httprate"
)

// LimitByIP 创建按 IP 地址限制的中间件。
// 参数：
//   - requests: 时间窗口内允许的最大请求数
//   - window: 时间窗口长度
//
// 返回：chi 中间件函数
func LimitByIP(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(
		requests,
		window,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
		}),
	)
}

// LimitByUserID 创建按用户 ID 限制的中间件。
// 用户 ID 从请求上下文的 CtxUserID 键获取（需要先经过 Authenticate 中间件）。
// 如果没有用户 ID，回退到按 IP 限制。
//
// 参数：
//   - requests: 时间窗口内允许的最大请求数
//   - window: 时间窗口长度
//
// 返回：chi 中间件函数
func LimitByUserID(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(
		requests,
		window,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			userID := UserID(r)
			if userID == "" {
				return httprate.KeyByIP(r)
			}
			return userID, nil
		}),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			logger.Warn("rate limited", "user", UserID(r), "ip", r.RemoteAddr, "endpoint", r.URL.Path)
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
		}),
	)
}
