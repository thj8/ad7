// Package auth 实现用户注册登录和队伍管理。
// 提供用户认证（注册、登录、JWT 生成）和队伍 CRUD 及成员管理功能。
package auth

import (
	"ad7/internal/model"
)

// User 表示系统用户。
// PasswordHash 使用 json:"-" 标签确保密码哈希不会出现在 API 响应中。
type User struct {
	model.BaseModel
	Username     string `json:"username"`      // 用户名（唯一）
	PasswordHash string `json:"-"`             // bcrypt 密码哈希，不暴露给 API
	Role         string `json:"role"`          // 角色（member / admin）
}

// Team 表示参赛队伍。
// 队伍可以包含多个用户，通过 team_members 关联。
type Team struct {
	model.BaseModel
	Name        string `json:"name"`        // 队伍名称（唯一）
	Description string `json:"description"` // 队伍描述
}

// TeamMember 表示队伍成员关系。
// 支持队伍内部角色（captain/member）。
type TeamMember struct {
	model.BaseModel
	TeamID string `json:"team_id"` // 队伍 res_id
	UserID string `json:"user_id"` // 用户 res_id
	Role   string `json:"role"`    // 角色：captain 或 member
}

// MemberInfo 表示队伍成员信息，用于 API 响应。
type MemberInfo struct {
	UserID   string `json:"user_id"`   // 用户 res_id
	Username string `json:"username"`  // 用户名
	Role     string `json:"role"`      // 角色：captain 或 member
	JoinedAt string `json:"joined_at"` // 加入时间（RFC3339 格式）
}
