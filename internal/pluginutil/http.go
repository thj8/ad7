package pluginutil

import (
	"encoding/json"
	"net/http"
)

// WriteJSON 将数据序列化为 JSON 并写入 HTTP 响应。
// 设置 Content-Type 为 application/json，并指定 HTTP 状态码。
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError 返回 JSON 格式的错误响应 {"error": "msg"}。
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}
