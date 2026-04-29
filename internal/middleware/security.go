// Package middleware 提供 HTTP 中间件。
package middleware

import (
	"net/http"
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

