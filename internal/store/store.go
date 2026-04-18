// Package store 定义数据访问层的接口。
// 接口按领域分为 ChallengeStore、SubmissionStore、CompetitionStore，
// 实现层在 mysql.go 中统一由 *Store 结构体实现。
package store

import (
	"context"

	"ad7/internal/model"
)

// ChallengeStore 定义题目相关的数据访问接口。
type ChallengeStore interface {
	// ListEnabled 查询所有已启用且未删除的题目。
	// 返回题目列表，不包含 Flag 字段。
	ListEnabled(ctx context.Context) ([]model.Challenge, error)

	// GetEnabledByID 根据 res_id 查询单个已启用且未删除的题目（含 Flag）。
	// 如果未找到返回 nil, nil。
	GetEnabledByID(ctx context.Context, resID string) (*model.Challenge, error)

	// GetByID 根据 res_id 查询单个未删除的题目（含 Flag），不检查启用状态。
	// 用于管理员更新题目时获取完整信息。如果未找到返回 nil, nil。
	GetByID(ctx context.Context, resID string) (*model.Challenge, error)

	// Create 创建新题目，自动生成 res_id。
	// 返回生成的 res_id。
	Create(ctx context.Context, c *model.Challenge) (string, error)

	// Update 根据 res_id 更新题目的全部字段。
	Update(ctx context.Context, c *model.Challenge) error

	// Delete 软删除题目，将 is_deleted 设为 1。
	Delete(ctx context.Context, resID string) error
}

// SubmissionStore 定义提交记录相关的数据访问接口。
type SubmissionStore interface {
	// HasCorrectSubmission 检查指定用户是否已正确提交过某道题目（全局范围）。
	// 用于防止重复提交。
	HasCorrectSubmission(ctx context.Context, userID string, challengeID string) (bool, error)

	// CreateSubmission 创建一条全局提交记录（不关联比赛）。
	CreateSubmission(ctx context.Context, s *model.Submission) error

	// ListSubmissions 根据用户 ID 和/或题目 ID 查询提交记录。
	// 两个参数均可为空字符串，表示不过滤该条件。按创建时间倒序排列。
	ListSubmissions(ctx context.Context, userID string, challengeID string) ([]model.Submission, error)

	// HasCorrectSubmissionInComp 检查指定用户在指定比赛中是否已正确提交过某道题目。
	// 用于比赛中防止重复提交。
	HasCorrectSubmissionInComp(ctx context.Context, userID string, challengeID, competitionID string) (bool, error)

	// CreateSubmissionWithComp 创建一条关联比赛的提交记录。
	CreateSubmissionWithComp(ctx context.Context, s *model.Submission) error

	// ListSubmissionsByComp 查询指定比赛内的提交记录。
	// 可通过用户 ID 和/或题目 ID 进一步过滤。按创建时间倒序排列。
	ListSubmissionsByComp(ctx context.Context, competitionID string, userID string, challengeID string) ([]model.Submission, error)
}

// CompetitionStore 定义比赛相关的数据访问接口。
type CompetitionStore interface {
	// ListCompetitions 查询所有未删除的比赛，按创建时间倒序排列。
	// 管理员使用，包含未激活的比赛。
	ListCompetitions(ctx context.Context) ([]model.Competition, error)

	// ListActiveCompetitions 查询所有已激活且未删除的比赛，按创建时间倒序排列。
	// 普通用户使用，只看到激活的比赛。
	ListActiveCompetitions(ctx context.Context) ([]model.Competition, error)

	// GetCompetitionByID 根据 res_id 查询单个未删除的比赛。
	// 如果未找到返回 nil, nil。
	GetCompetitionByID(ctx context.Context, resID string) (*model.Competition, error)

	// CreateCompetition 创建新比赛，自动生成 res_id。
	// 返回生成的 res_id。
	CreateCompetition(ctx context.Context, c *model.Competition) (string, error)

	// UpdateCompetition 根据 res_id 更新比赛信息。
	UpdateCompetition(ctx context.Context, c *model.Competition) error

	// DeleteCompetition 软删除比赛，同时删除该比赛的题目关联记录。
	DeleteCompetition(ctx context.Context, resID string) error

	// AddChallenge 将一道题目分配到比赛中，自动生成 res_id。
	AddChallenge(ctx context.Context, compID, chalID string) error

	// RemoveChallenge 从比赛中移除一道题目的关联。
	RemoveChallenge(ctx context.Context, compID, chalID string) error

	// ListCompChallenges 查询指定比赛中所有已启用且未删除的题目。
	// 通过 competition_challenges 关联表 JOIN 查询。
	ListCompChallenges(ctx context.Context, compID string) ([]model.Challenge, error)
}
