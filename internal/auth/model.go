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
	TeamID       string `json:"team_id,omitempty"` // 所属队伍的 res_id，可为空
}

// Team 表示参赛队伍。
// 队伍可以包含多个用户，通过 users.team_id 关联。
type Team struct {
	model.BaseModel
	Name        string `json:"name"`        // 队伍名称（唯一）
	Description string `json:"description"` // 队伍描述
}
