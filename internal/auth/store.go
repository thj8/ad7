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

	// ListUsersByTeam 查询指定队伍的所有用户。
	ListUsersByTeam(ctx context.Context, teamID string) ([]User, error)

	// SetTeamID 设置用户的队伍归属。
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

	// DeleteTeam 软删除队伍，同时清除该队伍所有用户的 team_id。
	DeleteTeam(ctx context.Context, resID string) error
}
