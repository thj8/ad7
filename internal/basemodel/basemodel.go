// Package basemodel 提供共享的基础模型类型和验证工具。
// 供 CTF 服务和 Auth 服务共同使用，不包含任何业务领域模型。
package basemodel

import (
	"database/sql/driver"
	"fmt"
	"time"
)

const timeFormat = "2006-01-02 15:04:05"

// ValidationError 表示字段验证失败错误。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ErrFieldRequired 返回字段必填错误。
func ErrFieldRequired(field string) error {
	return &ValidationError{Field: field, Message: "required"}
}

// ErrFieldTooLong 返回字段超长错误。
func ErrFieldTooLong(field string, max int) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf("too long (max %d)", max)}
}

// ErrFieldInvalid 返回字段无效错误。
func ErrFieldInvalid(field string, reason string) error {
	return &ValidationError{Field: field, Message: reason}
}

// Time 自定义时间类型，使用 "2006-01-02 15:04:05" 格式进行 JSON 序列化。
type Time time.Time

// MarshalJSON 实现 json.Marshaler 接口，使用自定义时间格式。
func (t Time) MarshalJSON() ([]byte, error) {
	if time.Time(t).IsZero() {
		return []byte(`null`), nil
	}
	return []byte(`"` + time.Time(t).Format(timeFormat) + `"`), nil
}

// UnmarshalJSON 实现 json.Unmarshaler 接口，支持解析自定义时间格式和 RFC3339。
func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		*t = Time{}
		return nil
	}
	// 先尝试我们的自定义格式
	parsed, err := time.Parse(`"`+timeFormat+`"`, string(data))
	if err == nil {
		*t = Time(parsed)
		return nil
	}
	// 也支持 RFC3339 格式，保持兼容性
	parsed, err = time.Parse(`"`+time.RFC3339+`"`, string(data))
	if err == nil {
		*t = Time(parsed)
		return nil
	}
	return fmt.Errorf("invalid time format: %s", data)
}

// Value 实现 driver.Valuer 接口，用于数据库写入。
func (t Time) Value() (driver.Value, error) {
	return time.Time(t), nil
}

// Scan 实现 sql.Scanner 接口，用于数据库读取。
func (t *Time) Scan(value interface{}) error {
	if value == nil {
		*t = Time{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*t = Time(v)
		return nil
	case []byte:
		parsed, err := time.Parse(timeFormat, string(v))
		if err == nil {
			*t = Time(parsed)
			return nil
		}
		parsed, err = time.Parse(time.RFC3339, string(v))
		if err == nil {
			*t = Time(parsed)
			return nil
		}
		return fmt.Errorf("invalid time value: %v", v)
	case string:
		parsed, err := time.Parse(timeFormat, v)
		if err == nil {
			*t = Time(parsed)
			return nil
		}
		parsed, err = time.Parse(time.RFC3339, v)
		if err == nil {
			*t = Time(parsed)
			return nil
		}
		return fmt.Errorf("invalid time value: %v", v)
	default:
		return fmt.Errorf("cannot scan %T to Time", value)
	}
}

// Time 返回底层的 time.Time
func (t Time) Time() time.Time {
	return time.Time(t)
}

// IsZero 代理 time.Time.IsZero
func (t Time) IsZero() bool {
	return time.Time(t).IsZero()
}

// Before 代理 time.Time.Before
func (t Time) Before(u time.Time) bool {
	return time.Time(t).Before(u)
}

// After 代理 time.Time.After
func (t Time) After(u time.Time) bool {
	return time.Time(t).After(u)
}

// Format 代理 time.Time.Format
func (t Time) Format(layout string) string {
	return time.Time(t).Format(layout)
}

// BaseModel 是所有实体的基础结构体，包含公共字段。
// 支持软删除（is_deleted），使用 res_id（UUID v4，32 字符十六进制）作为对外 ID。
type BaseModel struct {
	ID        int    `json:"-"`          // 自增主键，仅内部使用，不暴露给 API
	ResID     string `json:"id"`         // 对外公开的 UUID 标识（32 字符十六进制，无连字符）
	CreatedAt Time   `json:"created_at"` // 创建时间
	UpdatedAt Time   `json:"updated_at"` // 更新时间
	IsDeleted bool   `json:"-"`          // 软删除标记，不暴露给 API
}
