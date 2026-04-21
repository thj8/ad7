package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"ad7/internal/uuid"
)

// AuthStore 实现 UserStore 和 TeamStore 接口。
// 持有 *sql.DB 数据库连接，与主 Store 共享同一个连接。
type AuthStore struct {
	db *sql.DB
}

// NewAuthStore 创建 AuthStore 实例。
func NewAuthStore(db *sql.DB) *AuthStore {
	return &AuthStore{db: db}
}

// --- UserStore 实现 ---

// CreateUser 创建新用户，自动生成 res_id。
func (s *AuthStore) CreateUser(ctx context.Context, u *User) (string, error) {
	u.ResID = uuid.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (res_id, username, password_hash, role) VALUES (?, ?, ?, ?)`,
		u.ResID, u.Username, u.PasswordHash, u.Role)
	if err != nil {
		return "", fmt.Errorf("create user %s: %w", u.Username, err)
	}
	return u.ResID, nil
}

// GetUserByUsername 根据用户名查询用户（含密码哈希）。
func (s *AuthStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, username, password_hash, role, created_at, updated_at
		 FROM users WHERE username = ? AND is_deleted = 0`, username).
		Scan(&u.ResID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username %s: %w", username, err)
	}
	return &u, nil
}

// GetUserByID 根据 res_id 查询用户。
func (s *AuthStore) GetUserByID(ctx context.Context, resID string) (*User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, username, password_hash, role, created_at, updated_at
		 FROM users WHERE res_id = ? AND is_deleted = 0`, resID).
		Scan(&u.ResID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id %s: %w", resID, err)
	}
	return &u, nil
}

// ListUsersByTeam 查询指定队伍的所有用户。
// Deprecated: 使用 TeamMemberStore.ListTeamMembers 替代
func (s *AuthStore) ListUsersByTeam(ctx context.Context, teamID string) ([]User, error) {
	return []User{}, nil
}

// SetTeamID 设置用户的队伍归属。
// Deprecated: 使用 TeamMemberStore.AddMember/RemoveMember 替代
func (s *AuthStore) SetTeamID(ctx context.Context, userID, teamID string) error {
	return nil
}

// DeleteUser 软删除用户。
func (s *AuthStore) DeleteUser(ctx context.Context, resID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET is_deleted = 1 WHERE res_id = ? AND is_deleted = 0`, resID)
	if err != nil {
		return fmt.Errorf("delete user %s: %w", resID, err)
	}
	return nil
}

// --- TeamStore 实现 ---

// CreateTeam 创建新队伍，自动生成 res_id。
func (s *AuthStore) CreateTeam(ctx context.Context, t *Team) (string, error) {
	t.ResID = uuid.Next()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO teams (res_id, name, description) VALUES (?, ?, ?)`,
		t.ResID, t.Name, t.Description)
	if err != nil {
		return "", fmt.Errorf("create team %s: %w", t.Name, err)
	}
	return t.ResID, nil
}

// GetTeamByID 根据 res_id 查询队伍。
func (s *AuthStore) GetTeamByID(ctx context.Context, resID string) (*Team, error) {
	var t Team
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, name, description, created_at, updated_at
		 FROM teams WHERE res_id = ? AND is_deleted = 0`, resID).
		Scan(&t.ResID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get team by id %s: %w", resID, err)
	}
	return &t, nil
}

// ListTeams 查询所有未删除的队伍。
func (s *AuthStore) ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT res_id, name, description, created_at, updated_at
		 FROM teams WHERE is_deleted = 0 ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	defer rows.Close()
	var teams []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ResID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

// UpdateTeam 根据 res_id 更新队伍信息。
func (s *AuthStore) UpdateTeam(ctx context.Context, t *Team) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE teams SET name = ?, description = ? WHERE res_id = ? AND is_deleted = 0`,
		t.Name, t.Description, t.ResID)
	if err != nil {
		return fmt.Errorf("update team %s: %w", t.ResID, err)
	}
	return nil
}

// DeleteTeam 软删除队伍，同时软删除所有相关的 team_members 记录。
func (s *AuthStore) DeleteTeam(ctx context.Context, resID string) error {
	// 软删除该队伍所有的 team_members 记录
	if _, err := s.db.ExecContext(ctx,
		`UPDATE team_members SET is_deleted = 1 WHERE team_id = ? AND is_deleted = 0`, resID); err != nil {
		return fmt.Errorf("soft delete team members for team %s: %w", resID, err)
	}
	// 软删除队伍
	_, err := s.db.ExecContext(ctx,
		`UPDATE teams SET is_deleted = 1 WHERE res_id = ? AND is_deleted = 0`, resID)
	if err != nil {
		return fmt.Errorf("delete team %s: %w", resID, err)
	}
	return nil
}

// ListUsersByResIDs 根据 res_id 列表批量查询用户。
func (s *AuthStore) ListUsersByResIDs(ctx context.Context, resIDs []string) ([]User, error) {
	if len(resIDs) == 0 {
		return []User{}, nil
	}

	// 构建 IN 查询
	placeholders := make([]string, len(resIDs))
	args := make([]any, len(resIDs))
	for i, id := range resIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT res_id, username, password_hash, role, created_at, updated_at
		 FROM users WHERE res_id IN (%s) AND is_deleted = 0`,
		strings.Join(placeholders, ","),
	)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users by res_ids: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ResID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// --- TeamMemberStore 实现 ---

// AddMember 将用户添加到队伍，指定角色。
func (s *AuthStore) AddMember(ctx context.Context, teamID, userID, role string) (*TeamMember, error) {
	tm := &TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}
	tm.ResID = uuid.Next()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO team_members (res_id, team_id, user_id, role) VALUES (?, ?, ?, ?)`,
		tm.ResID, tm.TeamID, tm.UserID, tm.Role)
	if err != nil {
		return nil, fmt.Errorf("add team member %s to team %s: %w", userID, teamID, err)
	}

	// 读取刚创建的记录以获取完整信息
	return s.GetMember(ctx, teamID, userID)
}

// RemoveMember 从队伍中移除用户（软删除）。
func (s *AuthStore) RemoveMember(ctx context.Context, teamID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE team_members SET is_deleted = 1 WHERE team_id = ? AND user_id = ? AND is_deleted = 0`,
		teamID, userID)
	if err != nil {
		return fmt.Errorf("remove team member %s from team %s: %w", userID, teamID, err)
	}
	return nil
}

// GetMember 查询用户在指定队伍中的成员关系。
func (s *AuthStore) GetMember(ctx context.Context, teamID, userID string) (*TeamMember, error) {
	var tm TeamMember
	err := s.db.QueryRowContext(ctx,
		`SELECT id, res_id, team_id, user_id, role, created_at, updated_at
		 FROM team_members WHERE team_id = ? AND user_id = ? AND is_deleted = 0`,
		teamID, userID).
		Scan(&tm.ID, &tm.ResID, &tm.TeamID, &tm.UserID, &tm.Role, &tm.CreatedAt, &tm.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get team member %s in team %s: %w", userID, teamID, err)
	}
	return &tm, nil
}

// ListTeamMembers 查询指定队伍的所有成员。
func (s *AuthStore) ListTeamMembers(ctx context.Context, teamID string) ([]*TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, res_id, team_id, user_id, role, created_at, updated_at
		 FROM team_members WHERE team_id = ? AND is_deleted = 0 ORDER BY created_at ASC`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team members for team %s: %w", teamID, err)
	}
	defer rows.Close()

	var members []*TeamMember
	for rows.Next() {
		var tm TeamMember
		if err := rows.Scan(&tm.ID, &tm.ResID, &tm.TeamID, &tm.UserID, &tm.Role, &tm.CreatedAt, &tm.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		members = append(members, &tm)
	}
	return members, rows.Err()
}

// GetUserTeams 查询用户所属的所有队伍。
func (s *AuthStore) GetUserTeams(ctx context.Context, userID string) ([]*TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, res_id, team_id, user_id, role, created_at, updated_at
		 FROM team_members WHERE user_id = ? AND is_deleted = 0`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user teams for user %s: %w", userID, err)
	}
	defer rows.Close()

	var memberships []*TeamMember
	for rows.Next() {
		var tm TeamMember
		if err := rows.Scan(&tm.ID, &tm.ResID, &tm.TeamID, &tm.UserID, &tm.Role, &tm.CreatedAt, &tm.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user team: %w", err)
		}
		memberships = append(memberships, &tm)
	}
	return memberships, rows.Err()
}

// GetTeamMemberCount 查询指定队伍的成员数量。
func (s *AuthStore) GetTeamMemberCount(ctx context.Context, teamID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM team_members WHERE team_id = ? AND is_deleted = 0`, teamID).
		Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get team member count for team %s: %w", teamID, err)
	}
	return count, nil
}
