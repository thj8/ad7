// Package model 定义了 CTF 比赛平台的领域模型（实体结构体）。
// 所有实体都嵌入 BaseModel，使用 res_id（32 位十六进制 UUID）作为公开标识，
// 自增 id 仅用于内部，不会出现在 JSON 响应中。
package model

import "time"

// BaseModel 是所有实体的基础结构体，包含公共字段。
// 支持软删除（is_deleted），使用 res_id（UUID v4，32 字符十六进制）作为对外 ID。
type BaseModel struct {
	ID        int       `json:"-"`                // 自增主键，仅内部使用，不暴露给 API
	ResID     string    `json:"id"`               // 对外公开的 UUID 标识（32 字符十六进制，无连字符）
	CreatedAt time.Time `json:"created_at"`       // 创建时间
	UpdatedAt time.Time `json:"updated_at"`       // 更新时间
	IsDeleted bool      `json:"-"`                // 软删除标记，不暴露给 API
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
// 支持两种场景：全局提交（CompetitionID 为 nil）和比赛内提交。
type Submission struct {
	BaseModel
	UserID        string    `json:"user_id"`        // 提交用户 ID（来自 JWT 的 sub claim）
	ChallengeID   string    `json:"challenge_id"`   // 提交的题目 ID
	CompetitionID *string   `json:"competition_id"` // 所属比赛 ID，nil 表示全局提交
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
