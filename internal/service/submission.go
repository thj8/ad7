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
// 持有 ChallengeStore（用于验证题目和 Flag）、SubmissionStore（用于记录提交）、
// CompetitionStore（用于获取比赛模式）、TeamResolver（用于获取用户队伍）。
type SubmissionService struct {
	challenges  store.ChallengeStore
	submissions store.SubmissionStore
	competitions store.CompetitionStore
	teamResolver *TeamResolver
}

// NewSubmissionService 创建 SubmissionService 实例。
func NewSubmissionService(c store.ChallengeStore, s store.SubmissionStore, cs store.CompetitionStore, tr *TeamResolver) *SubmissionService {
	return &SubmissionService{challenges: c, submissions: s, competitions: cs, teamResolver: tr}
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
//  1. 获取比赛模式
//  2. 队伍模式：获取用户队伍，检查队伍是否已解决该题
//  3. 个人模式：检查用户是否已解决该题
//  4. 查询题目是否存在且启用
//  5. 比较提交的 Flag 与题目答案
//  6. 创建提交记录（包含 team_id，如果是队伍模式）
//  7. 如果正确，发布事件通知（供插件消费）
func (s *SubmissionService) SubmitInComp(ctx context.Context, req *SubmitInCompRequest) (SubmitResult, error) {
	// ========== 步骤 1：验证比赛信息 ==========
	comp, err := s.competitions.GetCompetitionByID(ctx, req.CompetitionID)
	if err != nil {
		return "", err
	}
	if comp == nil {
		return "", ErrNotFound
	}
	// 检查比赛是否在激活状态（IsActive 已包含时间范围检查）
	if !comp.IsActive {
		return "", ErrCompetitionNotActive
	}

	// ========== 步骤 2：检查用户/队伍是否已解决该题 ==========
	var teamID string
	if comp.Mode == model.CompetitionModeTeam {
		// ---------- 队伍模式分支 ----------
		// 获取用户所在队伍
		teamID, err = s.teamResolver.GetUserTeam(ctx, req.UserID)
		if err != nil {
			return "", err
		}
		if teamID == "" {
			return "", ErrMustJoinTeam
		}
		// 管理模式下检查队伍是否已注册到该比赛
		if comp.TeamJoinMode == model.TeamJoinModeManaged {
			inComp, err := s.competitions.IsTeamInComp(ctx, req.CompetitionID, teamID)
			if err != nil {
				return "", err
			}
			if !inComp {
				return "", ErrTeamNotRegistered
			}
		}
		// 检查队伍是否已正确提交过此题（队伍模式下同一题目只计一次）
		solved, err := s.submissions.HasTeamCorrectSubmission(ctx, teamID, req.ChallengeID, req.CompetitionID)
		if err != nil {
			return "", err
		}
		if solved {
			logger.Info("flag submitted", "user", req.UserID, "team", teamID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "already_solved")
			return ResultAlreadySolved, nil
		}
	} else {
		// ---------- 个人模式分支 ----------
		// 检查用户是否已正确提交过此题
		solved, err := s.submissions.HasCorrectSubmission(ctx, req.UserID, req.ChallengeID, req.CompetitionID)
		if err != nil {
			return "", err
		}
		if solved {
			logger.Info("flag submitted", "user", req.UserID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "already_solved")
			return ResultAlreadySolved, nil
		}
	}

	// ========== 步骤 3：验证题目信息 ==========
	challenge, err := s.challenges.GetEnabledByID(ctx, req.ChallengeID)
	if err != nil {
		return "", err
	}
	if challenge == nil {
		return "", ErrNotFound
	}

	// ========== 步骤 4：验证 Flag 并记录提交 ==========
	isCorrect := challenge.Flag == req.Flag
	// 创建提交记录（无论对错都记录，用于后续分析）
	if err := s.submissions.CreateSubmission(ctx, &model.Submission{
		UserID:        req.UserID,
		TeamID:        teamID,
		ChallengeID:   req.ChallengeID,
		CompetitionID: req.CompetitionID,
		SubmittedFlag: req.Flag,
		IsCorrect:     isCorrect,
	}); err != nil {
		return "", err
	}

	// ========== 步骤 5：正确提交后处理 ==========
	if isCorrect {
		// 发布正确提交事件，供插件（排行榜、一血、通知等）消费
		event.Publish(event.Event{
			Type:          event.EventCorrectSubmission,
			UserID:        req.UserID,
			TeamID:        teamID,
			ChallengeID:   req.ChallengeID,
			CompetitionID: req.CompetitionID,
			SubmittedAt:   time.Now(),
			Ctx:           ctx,
		})
		logger.Info("flag submitted", "user", req.UserID, "team", teamID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "correct")
		return ResultCorrect, nil
	}
	logger.Info("flag submitted", "user", req.UserID, "team", teamID, "challenge", req.ChallengeID, "competition", req.CompetitionID, "result", "incorrect")
	return ResultIncorrect, nil
}

// ListByComp 查询指定比赛内的提交记录。
// 可通过用户 ID 和题目 ID 进一步过滤。
func (s *SubmissionService) ListByComp(ctx context.Context, params store.ListSubmissionsParams) ([]model.Submission, error) {
	return s.submissions.ListSubmissions(ctx, params)
}
