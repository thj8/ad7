// Package service 实现比赛相关的业务逻辑。
// CompetitionService 处理比赛的 CRUD、题目分配等业务。
package service

import (
	"context"
	"errors"
	"time"

	"ad7/internal/logger"
	"ad7/internal/model"
	"ad7/internal/store"
)

// ErrConflict 表示操作冲突（如重复开始/结束比赛）。
var ErrConflict = errors.New("conflict")
var ErrCompetitionNotActive = errors.New("competition is not active")

// ErrMustJoinTeam 表示必须先加入队伍才能参加。
var ErrMustJoinTeam = errors.New("must join a team to participate")

// ErrTeamNotRegistered 表示队伍未注册到比赛。
var ErrTeamNotRegistered = errors.New("your team is not registered for this competition")

// ErrCompNotTeamMode 表示比赛不是队伍模式。
var ErrCompNotTeamMode = errors.New("competition is not in team mode")

// ErrCompFreeMode 表示比赛是自由模式，不能通过管理员添加队伍。
var ErrCompFreeMode = errors.New("competition uses free join mode")

// CompetitionService 封装比赛相关的业务逻辑。
// 持有 CompetitionStore 接口用于数据访问。
type CompetitionService struct {
	store store.CompetitionStore
}

// NewCompetitionService 创建 CompetitionService 实例。
// 参数 s: 实现 CompetitionStore 接口的数据访问层。
func NewCompetitionService(s store.CompetitionStore) *CompetitionService {
	return &CompetitionService{store: s}
}

// List 返回所有比赛（含未激活的），供管理员使用。
func (s *CompetitionService) List(ctx context.Context) ([]model.Competition, error) {
	cs, err := s.store.ListCompetitions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range cs {
		s.syncStatus(ctx, &cs[i])
	}
	return cs, nil
}

// ListActive 返回所有已激活的比赛，供普通用户查看。
func (s *CompetitionService) ListActive(ctx context.Context) ([]model.Competition, error) {
	cs, err := s.store.ListActiveCompetitions(ctx)
	if err != nil {
		return nil, err
	}
	var active []model.Competition
	for i := range cs {
		s.syncStatus(ctx, &cs[i])
		if cs[i].IsActive {
			active = append(active, cs[i])
		}
	}
	return active, nil
}

// Get 根据 res_id 获取单个比赛详情。
// 如果比赛不存在返回 ErrNotFound。
func (s *CompetitionService) Get(ctx context.Context, resID string) (*model.Competition, error) {
	c, err := s.store.GetCompetitionByID(ctx, resID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	s.syncStatus(ctx, c)
	return c, nil
}

// Create 创建新比赛。执行以下业务规则：
//   - title 为必填字段
//   - end_time 必须晚于 start_time
//   - mode 必须是 individual 或 team（默认为 individual）
//   - team_join_mode 必须是 free 或 managed（默认为 free）
//   - 新建比赛默认激活（is_active = true）
//
// 返回新生成比赛的 res_id。
func (s *CompetitionService) Create(ctx context.Context, c *model.Competition) (string, error) {
	// 设置默认模式
	if c.Mode == "" {
		c.Mode = model.CompetitionModeIndividual
	}
	if c.TeamJoinMode == "" {
		c.TeamJoinMode = model.TeamJoinModeFree
	}
	// 验证字段
	if err := c.Validate(); err != nil {
		return "", err
	}
	// 新建比赛默认激活
	c.IsActive = true
	return s.store.CreateCompetition(ctx, c)
}

// Update 使用合并策略更新比赛。只更新 patch 中非空/非零值的字段。
// 时间字段使用 IsZero() 判断是否需要更新（空字符串解析后为零值）。
// 更新后会再次验证 end_time >= start_time。
// is_active 字段总是被显式设置。
func (s *CompetitionService) Update(ctx context.Context, resID string, patch *model.Competition) error {
	// 先获取现有比赛
	existing, err := s.store.GetCompetitionByID(ctx, resID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	// 合并非空字段
	if patch.Title != "" {
		existing.Title = patch.Title
	}
	if patch.Description != "" {
		existing.Description = patch.Description
	}
	// 时间字段通过 IsZero 判断是否传入了新值
	if !patch.StartTime.IsZero() {
		existing.StartTime = patch.StartTime
	}
	if !patch.EndTime.IsZero() {
		existing.EndTime = patch.EndTime
	}
	// 合并模式（如果提供）
	if patch.Mode != "" {
		existing.Mode = patch.Mode
	}
	if patch.TeamJoinMode != "" {
		existing.TeamJoinMode = patch.TeamJoinMode
	}
	// is_active 总是被显式设置
	existing.IsActive = patch.IsActive
	// 验证合并后的字段
	if err := existing.Validate(); err != nil {
		return err
	}
	return s.store.UpdateCompetition(ctx, existing)
}

// CheckCompAccess 检查用户是否有访问比赛的权限。
// 个人模式：总是允许。
// 队伍模式-自由：用户必须有队伍。
// 队伍模式-管理：用户必须在比赛注册的队伍中。
func (s *CompetitionService) CheckCompAccess(ctx context.Context, compID, userID string, teamResolver *TeamResolver) error {
	c, err := s.store.GetCompetitionByID(ctx, compID)
	if err != nil {
		return err
	}
	if c == nil {
		return ErrNotFound
	}
	if c.Mode == model.CompetitionModeIndividual {
		return nil
	}
	teamID, err := teamResolver.GetUserTeam(ctx, userID)
	if err != nil {
		return err
	}
	if teamID == "" {
		return ErrMustJoinTeam
	}
	if c.TeamJoinMode == model.TeamJoinModeManaged {
		inComp, err := s.store.IsTeamInComp(ctx, compID, teamID)
		if err != nil {
			return err
		}
		if !inComp {
			return ErrTeamNotRegistered
		}
	}
	return nil
}

// AddCompTeam 将一支队伍加入比赛，仅在队伍模式-管理模式下允许。
func (s *CompetitionService) AddCompTeam(ctx context.Context, compID, teamID string) error {
	c, err := s.store.GetCompetitionByID(ctx, compID)
	if err != nil {
		return err
	}
	if c == nil {
		return ErrNotFound
	}
	if c.Mode != model.CompetitionModeTeam {
		return ErrCompNotTeamMode
	}
	if c.TeamJoinMode != model.TeamJoinModeManaged {
		return ErrCompFreeMode
	}
	return s.store.AddCompTeam(ctx, compID, teamID)
}

// RemoveCompTeam 将一支队伍从比赛中移除。
func (s *CompetitionService) RemoveCompTeam(ctx context.Context, compID, teamID string) error {
	c, err := s.store.GetCompetitionByID(ctx, compID)
	if err != nil {
		return err
	}
	if c == nil {
		return ErrNotFound
	}
	if c.Mode != model.CompetitionModeTeam {
		return ErrCompNotTeamMode
	}
	if c.TeamJoinMode != model.TeamJoinModeManaged {
		return ErrCompFreeMode
	}
	return s.store.RemoveCompTeam(ctx, compID, teamID)
}

// ListCompTeams 列出比赛中所有的队伍。
func (s *CompetitionService) ListCompTeams(ctx context.Context, compID string) ([]model.CompetitionTeam, error) {
	return s.store.ListCompTeams(ctx, compID)
}

// Delete 软删除比赛（同时删除题目关联记录）。
func (s *CompetitionService) Delete(ctx context.Context, resID string) error {
	return s.store.DeleteCompetition(ctx, resID)
}

// AddChallenge 将一道题目分配到比赛中。
func (s *CompetitionService) AddChallenge(ctx context.Context, compID, chalID string) error {
	return s.store.AddChallenge(ctx, compID, chalID)
}

// RemoveChallenge 从比赛中移除一道题目。
func (s *CompetitionService) RemoveChallenge(ctx context.Context, compID, chalID string) error {
	return s.store.RemoveChallenge(ctx, compID, chalID)
}

// ListChallenges 查询指定比赛中所有已启用的题目。
func (s *CompetitionService) ListChallenges(ctx context.Context, compID string) ([]model.Challenge, error) {
	return s.store.ListCompChallenges(ctx, compID)
}

// StartCompetition 手动开始比赛，设置 is_active = true。
// 如果比赛已激活返回 ErrConflict，不存在返回 ErrNotFound。
// 直接读取数据库，不经过 syncStatus，避免自动状态覆盖手动操作。
func (s *CompetitionService) StartCompetition(ctx context.Context, resID string) (*model.Competition, error) {
	c, err := s.store.GetCompetitionByID(ctx, resID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	if c.IsActive {
		return nil, ErrConflict
	}
	if err := s.store.SetActive(ctx, resID, true); err != nil {
		return nil, err
	}
	c.IsActive = true
	logger.Info("competition started", "competition_id", resID)
	return c, nil
}

// EndCompetition 手动结束比赛，设置 is_active = false。
// 如果比赛已结束返回 ErrConflict，不存在返回 ErrNotFound。
// 直接读取数据库，不经过 syncStatus，避免自动状态覆盖手动操作。
func (s *CompetitionService) EndCompetition(ctx context.Context, resID string) (*model.Competition, error) {
	c, err := s.store.GetCompetitionByID(ctx, resID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	if !c.IsActive {
		return nil, ErrConflict
	}
	if err := s.store.SetActive(ctx, resID, false); err != nil {
		return nil, err
	}
	c.IsActive = false
	logger.Info("competition ended", "competition_id", resID)
	return c, nil
}

// syncStatus 检查比赛时间并自动更新状态。
// 激活条件：start_time <= now && end_time > now && is_active == false
// 结束条件：end_time <= now && is_active == true
// 仅在状态实际变更时才写库，修改传入的 Competition 的 IsActive 字段。
func (s *CompetitionService) syncStatus(ctx context.Context, c *model.Competition) {
	now := time.Now()
	// 自动激活
	if !c.IsActive && !now.Before(c.StartTime.Time()) && now.Before(c.EndTime.Time()) {
		if err := s.store.SetActive(ctx, c.ResID, true); err != nil {
			logger.Error("failed to auto-activate competition", "competition_id", c.ResID, "error", err)
			return
		}
		c.IsActive = true
		logger.Info("competition auto-activated", "competition_id", c.ResID)
	}
	// 自动结束
	if c.IsActive && !now.Before(c.EndTime.Time()) {
		if err := s.store.SetActive(ctx, c.ResID, false); err != nil {
			logger.Error("failed to auto-end competition", "competition_id", c.ResID, "error", err)
			return
		}
		c.IsActive = false
		logger.Info("competition auto-ended", "competition_id", c.ResID)
	}
}
