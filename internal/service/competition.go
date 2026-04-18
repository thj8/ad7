// Package service 实现比赛相关的业务逻辑。
// CompetitionService 处理比赛的 CRUD、题目分配等业务。
package service

import (
	"context"
	"errors"

	"ad7/internal/model"
	"ad7/internal/store"
)

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
	return s.store.ListCompetitions(ctx)
}

// ListActive 返回所有已激活的比赛，供普通用户查看。
func (s *CompetitionService) ListActive(ctx context.Context) ([]model.Competition, error) {
	return s.store.ListActiveCompetitions(ctx)
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
	return c, nil
}

// Create 创建新比赛。执行以下业务规则：
//   - title 为必填字段
//   - end_time 必须晚于 start_time
//   - 新建比赛默认激活（is_active = true）
//
// 返回新生成比赛的 res_id。
func (s *CompetitionService) Create(ctx context.Context, c *model.Competition) (string, error) {
	// 验证必填字段
	if c.Title == "" {
		return "", errors.New("title is required")
	}
	// 验证时间合法性
	if c.EndTime.Before(c.StartTime) {
		return "", errors.New("end_time must be after start_time")
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
	// 合并后再次验证时间合法性
	if existing.EndTime.Before(existing.StartTime) {
		return errors.New("end_time must be after start_time")
	}
	// is_active 总是被显式设置
	existing.IsActive = patch.IsActive
	return s.store.UpdateCompetition(ctx, existing)
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
