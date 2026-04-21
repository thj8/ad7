package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUserAlreadyInTeam    = errors.New("user already belongs to another team")
	ErrUserAlreadyMember    = errors.New("user is already a member of this team")
	ErrCannotRemoveCaptain  = errors.New("cannot remove captain, transfer captain first")
	ErrUserNotMember        = errors.New("user is not a member of this team")
	ErrRoleCaptainRequired  = errors.New("captain role required")
)

// TeamService 处理队伍 CRUD 和成员管理业务逻辑。
type TeamService struct {
	teams        TeamStore
	users        UserStore
	teamMembers  TeamMemberStore
}

// NewTeamService 创建 TeamService 实例。
func NewTeamService(teams TeamStore, users UserStore, teamMembers TeamMemberStore) *TeamService {
	return &TeamService{
		teams:       teams,
		users:       users,
		teamMembers: teamMembers,
	}
}

// CreateTeam 创建新队伍（向后兼容，不设置创建者）。
func (s *TeamService) CreateTeam(ctx context.Context, t *Team) (string, error) {
	if t.Name == "" {
		return "", errors.New("team name is required")
	}
	if len(t.Name) > 255 {
		return "", errors.New("team name too long (max 255)")
	}
	return s.teams.CreateTeam(ctx, t)
}

// CreateTeamWithCreator 创建新队伍，创建者自动成为队长。
func (s *TeamService) CreateTeamWithCreator(ctx context.Context, t *Team, creatorUserID string) (string, error) {
	if t.Name == "" {
		return "", errors.New("team name is required")
	}
	if len(t.Name) > 255 {
		return "", errors.New("team name too long (max 255)")
	}

	teamID, err := s.teams.CreateTeam(ctx, t)
	if err != nil {
		return "", err
	}

	// 如果提供了创建者用户ID，将其添加为队长
	if creatorUserID != "" {
		_, err := s.teamMembers.AddMember(ctx, teamID, creatorUserID, "captain")
		if err != nil {
			// 回滚队伍创建
			_ = s.teams.DeleteTeam(ctx, teamID)
			return "", fmt.Errorf("add creator as captain: %w", err)
		}
	}

	return teamID, nil
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

// AddMember 将用户添加到队伍，role 可选，默认为 "member"。
func (s *TeamService) AddMember(ctx context.Context, teamID, userID, role string) error {
	if role == "" {
		role = "member"
	}
	if role != "captain" && role != "member" {
		return errors.New("invalid role, must be 'captain' or 'member'")
	}

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

	// 检查用户是否已在其他队伍中（单队约束）
	userTeams, err := s.teamMembers.GetUserTeams(ctx, userID)
	if err != nil {
		return err
	}
	if len(userTeams) > 0 {
		// 检查是否已在当前队伍中
		for _, tm := range userTeams {
			if tm.TeamID == teamID {
				return ErrUserAlreadyMember
			}
		}
		return ErrUserAlreadyInTeam
	}

	// 如果要添加为队长，检查当前是否已有队长
	if role == "captain" {
		members, err := s.teamMembers.ListTeamMembers(ctx, teamID)
		if err != nil {
			return err
		}
		for _, m := range members {
			if m.Role == "captain" {
				return errors.New("team already has a captain")
			}
		}
	}

	_, err = s.teamMembers.AddMember(ctx, teamID, userID, role)
	return err
}

// RemoveMember 将用户从队伍中移除。
func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID string) error {
	// 验证队伍存在
	team, err := s.teams.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}
	if team == nil {
		return ErrNotFound
	}

	// 获取用户的成员关系
	member, err := s.teamMembers.GetMember(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrUserNotMember
	}

	// 如果是队长，检查是否还有其他成员
	if member.Role == "captain" {
		count, err := s.teamMembers.GetTeamMemberCount(ctx, teamID)
		if err != nil {
			return err
		}
		if count > 1 {
			return ErrCannotRemoveCaptain
		}
	}

	return s.teamMembers.RemoveMember(ctx, teamID, userID)
}

// ListMembers 查询队伍的所有成员，返回 MemberInfo 数组。
func (s *TeamService) ListMembers(ctx context.Context, teamID string) ([]MemberInfo, error) {
	// 验证队伍存在
	team, err := s.teams.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, ErrNotFound
	}

	// 获取所有成员关系
	tmList, err := s.teamMembers.ListTeamMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if len(tmList) == 0 {
		return []MemberInfo{}, nil
	}

	// 批量获取用户信息
	userIDs := make([]string, len(tmList))
	for i, tm := range tmList {
		userIDs[i] = tm.UserID
	}
	users, err := s.users.ListUsersByResIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	// 构建用户 map
	userMap := make(map[string]*User)
	for i := range users {
		userMap[users[i].ResID] = &users[i]
	}

	// 组装 MemberInfo
	result := make([]MemberInfo, 0, len(tmList))
	for _, tm := range tmList {
		user, ok := userMap[tm.UserID]
		if !ok {
			continue // 跳过找不到的用户
		}
		result = append(result, MemberInfo{
			UserID:   tm.UserID,
			Username: user.Username,
			Role:     tm.Role,
			JoinedAt: tm.CreatedAt.Format(time.RFC3339),
		})
	}

	return result, nil
}

// SetCaptain 将指定用户提升为队长，原队长降为成员。
func (s *TeamService) SetCaptain(ctx context.Context, teamID, userID string) error {
	// 验证队伍存在
	team, err := s.teams.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}
	if team == nil {
		return ErrNotFound
	}

	// 验证目标用户是队伍成员
	targetMember, err := s.teamMembers.GetMember(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if targetMember == nil {
		return ErrUserNotMember
	}

	// 如果已经是队长，无需操作
	if targetMember.Role == "captain" {
		return nil
	}

	// 获取所有成员
	members, err := s.teamMembers.ListTeamMembers(ctx, teamID)
	if err != nil {
		return err
	}

	// 找到原队长并降级
	for _, m := range members {
		if m.Role == "captain" {
			// 先移除原队长再添加为 member（由于软删除，需要用 Remove 后 Add）
			if err := s.teamMembers.RemoveMember(ctx, teamID, m.UserID); err != nil {
				return err
			}
			if _, err := s.teamMembers.AddMember(ctx, teamID, m.UserID, "member"); err != nil {
				return err
			}
			break
		}
	}

	// 提升目标用户为队长
	if err := s.teamMembers.RemoveMember(ctx, teamID, userID); err != nil {
		return err
	}
	if _, err := s.teamMembers.AddMember(ctx, teamID, userID, "captain"); err != nil {
		return err
	}

	return nil
}

// TransferCaptain 显式转移队长权限。
func (s *TeamService) TransferCaptain(ctx context.Context, teamID, fromUserID, toUserID string) error {
	// 验证队伍存在
	team, err := s.teams.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}
	if team == nil {
		return ErrNotFound
	}

	// 验证 fromUserID 是当前队长
	fromMember, err := s.teamMembers.GetMember(ctx, teamID, fromUserID)
	if err != nil {
		return err
	}
	if fromMember == nil || fromMember.Role != "captain" {
		return errors.New("from_user is not the captain")
	}

	// 使用 SetCaptain 完成转移
	return s.SetCaptain(ctx, teamID, toUserID)
}
