// Package service 实现 Flag 提交相关的业务逻辑。
// SubmissionService 处理比赛内的 Flag 提交验证、防重复提交、事件发布等。
package service

import (
	"context"
	"time"

	"ad7/internal/event"
	"ad7/internal/logger"
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
func NewSubmissionService(c store.ChallengeStore, s store.SubmissionStore) *SubmissionService {
	return &SubmissionService{challenges: c, submissions: s}
}

// SubmitInCompRequest 是比赛内 Flag 提交的请求参数。
type SubmitInCompRequest struct {
	UserID        string // 当前用户 ID（来自 JWT）
	CompetitionID string // 比赛的 res_id
	ChallengeID   string // 题目的 res_id
	Flag          string // 用户提交的 Flag 字符串
}

// SubmitInComp 处理比赛范围内的 Flag 提交。
// 流程：
//  1. 检查用户在该比赛中是否已正确提交过该题目，如果是返回 ResultAlreadySolved
//  2. 查询题目是否存在且启用
//  3. 比较提交的 Flag 与题目答案
//  4. 创建提交记录
//  5. 如果正确，发布事件通知（供插件消费）
func (s *SubmissionService) SubmitInComp(ctx context.Context, req *SubmitInCompRequest) (SubmitResult, error) {
	solved, err := s.submissions.HasCorrectSubmission(ctx, req.UserID, req.ChallengeID, req.CompetitionID)
	if err != nil {
		return "", err
	}
	if solved {
		logger.Info("flag submitted", "user", req.UserID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "already_solved")
		return ResultAlreadySolved, nil
	}

	challenge, err := s.challenges.GetEnabledByID(ctx, req.ChallengeID)
	if err != nil {
		return "", err
	}
	if challenge == nil {
		return "", ErrNotFound
	}

	isCorrect := challenge.Flag == req.Flag
	if err := s.submissions.CreateSubmission(ctx, &model.Submission{
		UserID:        req.UserID,
		ChallengeID:   req.ChallengeID,
		CompetitionID: req.CompetitionID,
		SubmittedFlag: req.Flag,
		IsCorrect:     isCorrect,
	}); err != nil {
		return "", err
	}

	if isCorrect {
		event.Publish(event.Event{
			Type:          event.EventCorrectSubmission,
			UserID:        req.UserID,
			ChallengeID:   req.ChallengeID,
			CompetitionID: req.CompetitionID,
			SubmittedAt:   time.Now(),
			Ctx:           ctx,
		})
		logger.Info("flag submitted", "user", req.UserID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "correct")
		return ResultCorrect, nil
	}
	logger.Info("flag submitted", "user", req.UserID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "incorrect")
	return ResultIncorrect, nil
}

// ListByComp 查询指定比赛内的提交记录。
// 可通过用户 ID 和题目 ID 进一步过滤。
func (s *SubmissionService) ListByComp(ctx context.Context, params store.ListSubmissionsParams) ([]model.Submission, error) {
	return s.submissions.ListSubmissions(ctx, params)
}
