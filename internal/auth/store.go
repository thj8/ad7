package auth

import "context"

// UserStore 定义用户相关的数据访问接口。
type UserStore interface {
	// CreateUser 创建新用户。返回生成的 res_id。
	CreateUser(ctx context.Context, u *User) (string, error)

	// GetUserByUsername 根据用户名查询用户（含密码哈希）。
	// 用于登录验证。如果未找到返回 nil, nil。
	GetUserByUsername(ctx context.Context, username string) (*User, error)

	// GetUserByID 根据 res_id 查询用户。如果未找到返回 nil, nil。
	GetUserByID(ctx context.Context, resID string) (*User, error)

	// ListUsersByResIDs 根据 res_id 列表批量查询用户。
	ListUsersByResIDs(ctx context.Context, resIDs []string) ([]User, error)

	// ListUsersByTeam 查询指定队伍的所有用户。
	// Deprecated: 使用 TeamMemberStore.ListTeamMembers 替代
	ListUsersByTeam(ctx context.Context, teamID string) ([]User, error)

	// SetTeamID 设置用户的队伍归属。
	// Deprecated: 使用 TeamMemberStore.AddMember/RemoveMember 替代
	SetTeamID(ctx context.Context, userID, teamID string) error

	// DeleteUser 软删除用户。
	DeleteUser(ctx context.Context, resID string) error
}

// TeamStore 定义队伍相关的数据访问接口。
type TeamStore interface {
	// CreateTeam 创建新队伍。返回生成的 res_id。
	CreateTeam(ctx context.Context, t *Team) (string, error)

	// GetTeamByID 根据 res_id 查询队伍。如果未找到返回 nil, nil。
	GetTeamByID(ctx context.Context, resID string) (*Team, error)

	// ListTeams 查询所有未删除的队伍。
	ListTeams(ctx context.Context) ([]Team, error)

	// UpdateTeam 根据 res_id 更新队伍信息。
	UpdateTeam(ctx context.Context, t *Team) error

	// DeleteTeam 软删除队伍，同时软删除所有相关的 team_members 记录。
	DeleteTeam(ctx context.Context, resID string) error
}

// TeamMemberStore 定义队伍成员关系的数据访问接口。
type TeamMemberStore interface {
	// AddMember 将用户添加到队伍，指定角色。
	AddMember(ctx context.Context, teamID, userID, role string) (*TeamMember, error)

	// RemoveMember 从队伍中移除用户（软删除）。
	RemoveMember(ctx context.Context, teamID, userID string) error

	// GetMember 查询用户在指定队伍中的成员关系。
	GetMember(ctx context.Context, teamID, userID string) (*TeamMember, error)

	// ListTeamMembers 查询指定队伍的所有成员。
	ListTeamMembers(ctx context.Context, teamID string) ([]*TeamMember, error)

	// GetUserTeams 查询用户所属的所有队伍。
	GetUserTeams(ctx context.Context, userID string) ([]*TeamMember, error)

	// GetTeamMemberCount 查询指定队伍的成员数量。
	GetTeamMemberCount(ctx context.Context, teamID string) (int, error)
}
