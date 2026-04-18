package pluginutil

import "fmt"

// ParseID 校验 res_id 是否为有效的 32 字符十六进制字符串。
// 不合法时返回错误，调用方可直接用于 400 响应。
func ParseID(id string) error {
	if len(id) != 32 {
		return fmt.Errorf("invalid id: %q", id)
	}
	return nil
}
