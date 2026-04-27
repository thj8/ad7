package pluginutil

import "ad7/internal/uuid"

// ParseID 校验 res_id 是否为有效的 32 字符十六进制字符串。
// 不合法时返回错误，调用方可直接用于 400 响应。
// Deprecated: Use uuid.Validate instead.
func ParseID(id string) error {
	return uuid.Validate(id)
}
