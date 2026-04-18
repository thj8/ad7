// Package service 实现业务逻辑层。
// ChallengeService 处理题目的 CRUD 业务逻辑，包括输入验证、默认值填充和合并更新。
package service

import (
	"context"
	"errors"

	"ad7/internal/model"
	"ad7/internal/store"
)

// ErrNotFound 是通用的"资源未找到"错误，供 Handler 层判断返回 404。
var ErrNotFound = errors.New("not found")

// ChallengeService 封装题目相关的业务逻辑。
// 持有 ChallengeStore 接口用于数据访问。
type ChallengeService struct {
	store store.ChallengeStore
}

// NewChallengeService 创建 ChallengeService 实例。
// 参数 s: 实现 ChallengeStore 接口的数据访问层。
func NewChallengeService(s store.ChallengeStore) *ChallengeService {
	return &ChallengeService{store: s}
}

// List 返回所有已启用的题目列表。
// 直接委托给 Store 层查询。
func (s *ChallengeService) List(ctx context.Context) ([]model.Challenge, error) {
	return s.store.ListEnabled(ctx)
}

// Get 根据 res_id 获取单个已启用的题目详情（含 Flag，但不通过 API 返回）。
// 如果题目不存在返回 ErrNotFound。
func (s *ChallengeService) Get(ctx context.Context, resID string) (*model.Challenge, error) {
	c, err := s.store.GetEnabledByID(ctx, resID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	return c, nil
}

// Create 创建新题目。执行以下业务规则：
//   - title 和 flag 为必填字段
//   - score 未设置时默认 100
//   - category 未设置时默认 "misc"
//   - 新建题目默认启用（is_enabled = true）
//
// 返回新生成题目的 res_id。
func (s *ChallengeService) Create(ctx context.Context, c *model.Challenge) (string, error) {
	// 验证必填字段
	if c.Title == "" || c.Flag == "" {
		return "", errors.New("title and flag are required")
	}
	// 设置默认分值
	if c.Score <= 0 {
		c.Score = 100
	}
	// 设置默认分类
	if c.Category == "" {
		c.Category = "misc"
	}
	// 新建题目默认启用
	c.IsEnabled = true
	return s.store.Create(ctx, c)
}

// Update 使用合并策略更新题目。只更新 patch 中非空/非零值的字段。
// 对于 is_enabled 字段，PUT 请求会显式设置（包括设为 false）。
// 如果目标题目不存在返回 ErrNotFound。
func (s *ChallengeService) Update(ctx context.Context, resID string, patch *model.Challenge) error {
	// 先获取现有题目
	existing, err := s.store.GetByID(ctx, resID)
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
	if patch.Category != "" {
		existing.Category = patch.Category
	}
	if patch.Description != "" {
		existing.Description = patch.Description
	}
	if patch.Score > 0 {
		existing.Score = patch.Score
	}
	if patch.Flag != "" {
		existing.Flag = patch.Flag
	}
	// PUT 请求总是显式设置 is_enabled，不使用合并策略
	existing.IsEnabled = patch.IsEnabled
	return s.store.Update(ctx, existing)
}

// Delete 软删除题目（将 is_deleted 设为 1）。
func (s *ChallengeService) Delete(ctx context.Context, resID string) error {
	return s.store.Delete(ctx, resID)
}
