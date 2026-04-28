// Package middleware 提供 HTTP 中间件。
package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// MaxBodySize 限制请求体大小。
// 超过限制返回 413 Request Entity Too Large。
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// LimitAuthEndpoints 创建认证端点的限流中间件。
// 对登录和注册端点使用更严格的限制。
func LimitAuthEndpoints(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(
		requests,
		window,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
		}),
	)
}
