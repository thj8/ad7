// Package service 实现 Flag 提交相关的业务逻辑。
// SubmissionService 处理全局和比赛内的 Flag 提交验证、防重复提交、事件发布等。
package service

import (
	"context"

	"ad7/internal/event"
	"ad7/internal/model"
	"ad7/internal/store"
)

// SubmitResult 是提交结果的字符串枚举类型。
type SubmitResult string

const (
	// ResultCorrect 表示 Flag 正确
	ResultCorrect SubmitResult = "correct"
	// ResultIncorrect 表示 Flag 错误
	ResultIncorrect SubmitResult = "incorrect"
	// ResultAlreadySolved 表示用户已正确提交过此题，无需重复提交
	ResultAlreadySolved SubmitResult = "already_solved"
)

// SubmissionService 封装 Flag 提交相关的业务逻辑。
// 持有 ChallengeStore（用于验证题目和 Flag）和 SubmissionStore（用于记录提交）。
type SubmissionService struct {
	challenges  store.ChallengeStore
	submissions store.SubmissionStore
}

// NewSubmissionService 创建 SubmissionService 实例。
// 参数 c: 题目数据访问接口；参数 s: 提交记录数据访问接口。
func NewSubmissionService(c store.ChallengeStore, s store.SubmissionStore) *SubmissionService {
	return &SubmissionService{challenges: c, submissions: s}
}

// Submit 处理全局范围内的 Flag 提交。
// 流程：
//  1. 检查用户是否已正确提交过该题目，如果是返回 ResultAlreadySolved
//  2. 查询题目是否存在且启用
//  3. 比较提交的 Flag 与题目答案
//  4. 创建提交记录
//  5. 如果正确，发布事件通知（供插件消费）
//
// 参数：
//   - ctx: 请求上下文
//   - userID: 当前用户 ID（来自 JWT）
//   - challengeID: 题目的 res_id
//   - flag: 用户提交的 Flag 字符串
//
// 返回提交结果和可能的错误。
func (s *SubmissionService) Submit(ctx context.Context, userID string, challengeID string, flag string) (SubmitResult, error) {
	// 检查是否已正确提交过（防重复）
	solved, err := s.submissions.HasCorrectSubmission(ctx, userID, challengeID)
	if err != nil {
		return "", err
	}
	if solved {
		return ResultAlreadySolved, nil
	}

	// 查询题目（需已启用且未删除）
	challenge, err := s.challenges.GetEnabledByID(ctx, challengeID)
	if err != nil {
		return "", err
	}
	if challenge == nil {
		return "", ErrNotFound
	}

	// 比较 Flag 是否正确（精确匹配）
	isCorrect := challenge.Flag == flag
	if err := s.submissions.CreateSubmission(ctx, &model.Submission{
		UserID:        userID,
		ChallengeID:   challengeID,
		SubmittedFlag: flag,
		IsCorrect:     isCorrect,
	}); err != nil {
		return "", err
	}

	// 正确时发布事件，供排行榜、仪表盘等插件消费
	if isCorrect {
		event.Publish(event.Event{
			Type:          event.EventCorrectSubmission,
			UserID:        userID,
			ChallengeID:   challengeID,
			CompetitionID: nil, // 全局提交无比赛 ID
			Ctx:           ctx,
		})
		return ResultCorrect, nil
	}
	return ResultIncorrect, nil
}

// List 查询全局范围内的提交记录。
// 参数均可为空字符串表示不过滤。
func (s *SubmissionService) List(ctx context.Context, userID string, challengeID string) ([]model.Submission, error) {
	return s.submissions.ListSubmissions(ctx, store.ListSubmissionsParams{UserID: userID, ChallengeID: challengeID})
}

// SubmitInCompRequest 是比赛内 Flag 提交的请求参数。
type SubmitInCompRequest struct {
	UserID        string // 当前用户 ID（来自 JWT）
	CompetitionID string // 比赛的 res_id
	ChallengeID   string // 题目的 res_id
	Flag          string // 用户提交的 Flag 字符串
}

// SubmitInComp 处理比赛范围内的 Flag 提交。
// 与 Submit 的区别在于：
//   - 在比赛范围内检查是否已正确提交（而非全局范围）
//   - 提交记录关联到比赛
//   - 发布的事件包含比赛 ID
//
// 返回提交结果和可能的错误。
func (s *SubmissionService) SubmitInComp(ctx context.Context, req *SubmitInCompRequest) (SubmitResult, error) {
	// 在比赛范围内检查是否已正确提交过
	solved, err := s.submissions.HasCorrectSubmission(ctx, req.UserID, req.ChallengeID, req.CompetitionID)
	if err != nil {
		return "", err
	}
	if solved {
		return ResultAlreadySolved, nil
	}

	// 查询题目
	challenge, err := s.challenges.GetEnabledByID(ctx, req.ChallengeID)
	if err != nil {
		return "", err
	}
	if challenge == nil {
		return "", ErrNotFound
	}

	// 比较 Flag
	isCorrect := challenge.Flag == req.Flag
	compID := req.CompetitionID
	if err := s.submissions.CreateSubmission(ctx, &model.Submission{
		UserID:        req.UserID,
		ChallengeID:   req.ChallengeID,
		CompetitionID: &compID,
		SubmittedFlag: req.Flag,
		IsCorrect:     isCorrect,
	}); err != nil {
		return "", err
	}

	// 正确时发布带比赛 ID 的事件
	if isCorrect {
		event.Publish(event.Event{
			Type:          event.EventCorrectSubmission,
			UserID:        req.UserID,
			ChallengeID:   req.ChallengeID,
			CompetitionID: &compID,
			Ctx:           ctx,
		})
		return ResultCorrect, nil
	}
	return ResultIncorrect, nil
}

// ListByComp 查询指定比赛内的提交记录。
// 可通过用户 ID 和题目 ID 进一步过滤。
func (s *SubmissionService) ListByComp(ctx context.Context, competitionID string, userID string, challengeID string) ([]model.Submission, error) {
	return s.submissions.ListSubmissions(ctx, store.ListSubmissionsParams{CompetitionID: competitionID, UserID: userID, ChallengeID: challengeID})
}
