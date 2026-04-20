package auth

import (
	"context"
	"errors"
	"fmt"
)

// TeamService 处理队伍 CRUD 和成员管理业务逻辑。
type TeamService struct {
	teams TeamStore
	users UserStore
}

// NewTeamService 创建 TeamService 实例。
func NewTeamService(teams TeamStore, users UserStore) *TeamService {
	return &TeamService{teams: teams, users: users}
}

// CreateTeam 创建新队伍。
func (s *TeamService) CreateTeam(ctx context.Context, t *Team) (string, error) {
	if t.Name == "" {
		return "", errors.New("team name is required")
	}
	if len(t.Name) > 255 {
		return "", errors.New("team name too long (max 255)")
	}
	return s.teams.CreateTeam(ctx, t)
}

// GetTeam 根据 res_id 查询队伍。
func (s *TeamService) GetTeam(ctx context.Context, resID string) (*Team, error) {
	t, err := s.teams.GetTeamByID(ctx, resID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, ErrNotFound
	}
	return t, nil
}

// ListTeams 查询所有队伍。
func (s *TeamService) ListTeams(ctx context.Context) ([]Team, error) {
	return s.teams.ListTeams(ctx)
}

// UpdateTeam 更新队伍信息。
func (s *TeamService) UpdateTeam(ctx context.Context, t *Team) error {
	if t.Name == "" {
		return errors.New("team name is required")
	}
	return s.teams.UpdateTeam(ctx, t)
}

// DeleteTeam 软删除队伍。
func (s *TeamService) DeleteTeam(ctx context.Context, resID string) error {
	return s.teams.DeleteTeam(ctx, resID)
}

// AddMember 将用户添加到队伍。
func (s *TeamService) AddMember(ctx context.Context, teamID, userID string) error {
	// 验证队伍存在
	team, err := s.teams.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}
	if team == nil {
		return ErrNotFound
	}
	// 验证用户存在
	user, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user %s: %w", userID, ErrNotFound)
	}
	return s.users.SetTeamID(ctx, userID, teamID)
}

// RemoveMember 将用户从队伍中移除。
func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID string) error {
	return s.users.SetTeamID(ctx, userID, "")
}

// ListMembers 查询队伍的所有成员。
func (s *TeamService) ListMembers(ctx context.Context, teamID string) ([]User, error) {
	team, err := s.teams.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, ErrNotFound
	}
	return s.users.ListUsersByTeam(ctx, teamID)
}
