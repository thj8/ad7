// Package model 定义了 CTF 比赛平台的领域模型（实体结构体）。
// 所有实体都嵌入 BaseModel，使用 res_id（32 位十六进制 UUID）作为公开标识，
// 自增 id 仅用于内部，不会出现在 JSON 响应中。
package model

import (
	"database/sql/driver"
	"fmt"
	"time"
)

const timeFormat = "2006-01-02 15:04:05"

// 验证错误
var (
	ErrInvalidMode = ErrFieldInvalid("mode", "must be individual or team")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func ErrFieldRequired(field string) error {
	return &ValidationError{Field: field, Message: "required"}
}

func ErrFieldTooLong(field string, max int) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf("too long (max %d)", max)}
}

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
	ID        int    `json:"-"`                // 自增主键，仅内部使用，不暴露给 API
	ResID     string `json:"id"`               // 对外公开的 UUID 标识（32 字符十六进制，无连字符）
	CreatedAt Time   `json:"created_at"`       // 创建时间
	UpdatedAt Time   `json:"updated_at"`       // 更新时间
	IsDeleted bool   `json:"-"`                // 软删除标记，不暴露给 API
}

// Challenge 表示一道 CTF 题目。
// Flag 字段使用 json:"-" 标签，确保 Flag 永远不会出现在 API 响应中，
// 防止题目答案泄露。Handler 层使用单独的请求结构体来接收 Flag。
type Challenge struct {
	BaseModel
	Title       string `json:"title"`        // 题目标题
	Category    string `json:"category"`     // 题目分类（web/pwn/reverse/crypto/misc）
	Description string `json:"description"`  // 题目描述
	Score       int    `json:"score"`        // 题目分值
	Flag        string `json:"-"`            // 题目答案，json:"-" 确保 API 响应中不包含此字段
	IsEnabled   bool   `json:"is_enabled"`   // 题目是否启用
}

// Submission 表示一次 Flag 提交记录。
// 所有提交都关联到比赛，CompetitionID 为必填字段。
type Submission struct {
	BaseModel
	UserID        string    `json:"user_id"`        // 提交用户 ID（来自 JWT 的 sub claim）
	TeamID        string    `json:"team_id"`        // 队伍 ID（队伍模式时填写）
	ChallengeID   string    `json:"challenge_id"`   // 提交的题目 ID
	CompetitionID string    `json:"competition_id"` // 所属比赛 ID
	SubmittedFlag string    `json:"submitted_flag"` // 用户提交的 Flag 内容
	IsCorrect     bool      `json:"is_correct"`     // 提交是否正确
}

// Notification 表示比赛通知。
// 每条通知关联一个比赛，由管理员创建，所有参赛用户可查看。
type Notification struct {
	BaseModel
	CompetitionID string `json:"competition_id"` // 所属比赛的 res_id
	Title         string `json:"title"`          // 通知标题
	Message       string `json:"message"`        // 通知内容
}

// Validate 验证 Challenge 字段
func (c *Challenge) Validate() error {
	if c.Title == "" {
		return ErrFieldRequired("title")
	}
	if len(c.Title) > 255 {
		return ErrFieldTooLong("title", 255)
	}
	if c.Category == "" {
		return ErrFieldRequired("category")
	}
	if len(c.Description) > 4096 {
		return ErrFieldTooLong("description", 4096)
	}
	if c.Score <= 0 {
		return ErrFieldInvalid("score", "must be positive")
	}
	if c.Flag == "" {
		return ErrFieldRequired("flag")
	}
	if len(c.Flag) > 255 {
		return ErrFieldTooLong("flag", 255)
	}
	return nil
}

// Validate 验证 Submission 字段
func (s *Submission) Validate() error {
	if s.UserID == "" {
		return ErrFieldRequired("user_id")
	}
	if len(s.UserID) > 32 {
		return ErrFieldTooLong("user_id", 32)
	}
	if s.TeamID != "" && len(s.TeamID) > 32 {
		return ErrFieldTooLong("team_id", 32)
	}
	if s.ChallengeID == "" {
		return ErrFieldRequired("challenge_id")
	}
	if len(s.ChallengeID) > 32 {
		return ErrFieldTooLong("challenge_id", 32)
	}
	if s.CompetitionID == "" {
		return ErrFieldRequired("competition_id")
	}
	if len(s.CompetitionID) > 32 {
		return ErrFieldTooLong("competition_id", 32)
	}
	if s.SubmittedFlag == "" {
		return ErrFieldRequired("submitted_flag")
	}
	if len(s.SubmittedFlag) > 255 {
		return ErrFieldTooLong("submitted_flag", 255)
	}
	return nil
}

// Validate 验证 Notification 字段
func (n *Notification) Validate() error {
	if n.CompetitionID == "" {
		return ErrFieldRequired("competition_id")
	}
	if len(n.CompetitionID) > 32 {
		return ErrFieldTooLong("competition_id", 32)
	}
	if n.Title == "" {
		return ErrFieldRequired("title")
	}
	if len(n.Title) > 255 {
		return ErrFieldTooLong("title", 255)
	}
	if n.Message == "" {
		return ErrFieldRequired("message")
	}
	if len(n.Message) > 4096 {
		return ErrFieldTooLong("message", 4096)
	}
	return nil
}
