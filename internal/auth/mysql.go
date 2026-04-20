package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
		`INSERT INTO users (res_id, username, password_hash, role, team_id) VALUES (?, ?, ?, ?, ?)`,
		u.ResID, u.Username, u.PasswordHash, u.Role, u.TeamID)
	if err != nil {
		return "", fmt.Errorf("create user %s: %w", u.Username, err)
	}
	return u.ResID, nil
}

// GetUserByUsername 根据用户名查询用户（含密码哈希）。
func (s *AuthStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	var teamID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, username, password_hash, role, team_id, created_at, updated_at
		 FROM users WHERE username = ? AND is_deleted = 0`, username).
		Scan(&u.ResID, &u.Username, &u.PasswordHash, &u.Role, &teamID, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username %s: %w", username, err)
	}
	u.TeamID = teamID.String
	return &u, nil
}

// GetUserByID 根据 res_id 查询用户。
func (s *AuthStore) GetUserByID(ctx context.Context, resID string) (*User, error) {
	var u User
	var teamID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT res_id, username, password_hash, role, team_id, created_at, updated_at
		 FROM users WHERE res_id = ? AND is_deleted = 0`, resID).
		Scan(&u.ResID, &u.Username, &u.PasswordHash, &u.Role, &teamID, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id %s: %w", resID, err)
	}
	u.TeamID = teamID.String
	return &u, nil
}

// ListUsersByTeam 查询指定队伍的所有用户。
func (s *AuthStore) ListUsersByTeam(ctx context.Context, teamID string) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT res_id, username, role, team_id, created_at, updated_at
		 FROM users WHERE team_id = ? AND is_deleted = 0 ORDER BY created_at ASC`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list users by team %s: %w", teamID, err)
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ResID, &u.Username, &u.Role, &u.TeamID, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SetTeamID 设置用户的队伍归属。teamID 为空字符串时清除队伍关联。
func (s *AuthStore) SetTeamID(ctx context.Context, userID, teamID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET team_id = ? WHERE res_id = ? AND is_deleted = 0`,
		sql.NullString{String: teamID, Valid: teamID != ""}, userID)
	if err != nil {
		return fmt.Errorf("set team for user %s: %w", userID, err)
	}
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

// DeleteTeam 软删除队伍，同时清除该队伍所有用户的 team_id。
func (s *AuthStore) DeleteTeam(ctx context.Context, resID string) error {
	// 清除该队伍所有用户的 team_id
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET team_id = NULL WHERE team_id = ? AND is_deleted = 0`, resID); err != nil {
		return fmt.Errorf("clear team members for team %s: %w", resID, err)
	}
	// 软删除队伍
	_, err := s.db.ExecContext(ctx,
		`UPDATE teams SET is_deleted = 1 WHERE res_id = ? AND is_deleted = 0`, resID)
	if err != nil {
		return fmt.Errorf("delete team %s: %w", resID, err)
	}
	return nil
}
