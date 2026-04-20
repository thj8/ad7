// Package uuid 提供 UUID v4 生成器。
// 生成的 UUID 为 32 字符十六进制字符串（无连字符），用作实体的公开 ID（res_id）。
package uuid

import (
	"crypto/rand"
	"fmt"
)

// Next 生成一个新的 UUID v4（32 字符十六进制字符串，无连字符）。
// 使用 crypto/rand 生成随机字节，设置版本号（4）和变体（RFC4122）。
// 返回格式示例：a1b2c3d4e5f67890a1b2c3d4e5f67890
func Next() string {
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	// 设置版本号为 4（随机生成），即第 7 字节高 4 位为 0100
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// 设置变体为 RFC4122，即第 9 字节高 2 位为 10
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	// 格式化为 32 字符十六进制字符串（无连字符）
	// 按照 UUID 标准分段格式化：4-2-2-2-6 字节
	return fmt.Sprintf("%x%x%x%x%x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)
}
