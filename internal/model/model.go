// Package model 定义了 CTF 比赛平台的领域模型（实体结构体）。
// 所有实体都嵌入 BaseModel，使用 res_id（32 位十六进制 UUID）作为公开标识，
// 自增 id 仅用于内部，不会出现在 JSON 响应中。
//
// 共享基础类型（BaseModel、Time、ValidationError 等）定义在 basemodel 包中，
// 本包通过类型别名重新导出，保持向后兼容。
package model

import (
	"ad7/internal/basemodel"
)

// 共享类型重新导出，保持向后兼容。
type BaseModel = basemodel.BaseModel
type Time = basemodel.Time
type ValidationError = basemodel.ValidationError

var (
	ErrFieldRequired = basemodel.ErrFieldRequired
	ErrFieldTooLong  = basemodel.ErrFieldTooLong
	ErrFieldInvalid  = basemodel.ErrFieldInvalid
)

// 验证错误
var (
	ErrInvalidMode = ErrFieldInvalid("mode", "must be individual or team")
)

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
