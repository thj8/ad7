// Package handler 提供 HTTP 响应工具函数和输入验证辅助。
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// writeJSON 将数据序列化为 JSON 并写入 HTTP 响应。
// 设置 Content-Type 为 application/json，并指定 HTTP 状态码。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeJSON 的错误响应封装。返回 JSON 格式的错误信息 {"error": "msg"}。
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// 字段长度常量
const (
	maxFieldLen = 4096
	maxTitleLen = 255
	maxFlagLen  = 255
)

// validateLen 验证字符串字段是否超过最大长度限制。
// 如果超过限制返回错误，否则返回 nil。
func validateLen(field, value string, max int) error {
	if len(value) > max {
		return fmt.Errorf("%s too long (max %d)", field, max)
	}
	return nil
}
