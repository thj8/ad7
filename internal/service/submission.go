package service

import (
	"context"

	"ad7/internal/event"
	"ad7/internal/model"
	"ad7/internal/store"
)

type SubmitResult string

const (
	ResultCorrect       SubmitResult = "correct"
	ResultIncorrect     SubmitResult = "incorrect"
	ResultAlreadySolved SubmitResult = "already_solved"
)

type SubmissionService struct {
	challenges  store.ChallengeStore
	submissions store.SubmissionStore
}

func NewSubmissionService(c store.ChallengeStore, s store.SubmissionStore) *SubmissionService {
	return &SubmissionService{challenges: c, submissions: s}
}

func (s *SubmissionService) Submit(ctx context.Context, userID string, challengeID string, flag string) (SubmitResult, error) {
	solved, err := s.submissions.HasCorrectSubmission(ctx, userID, challengeID)
	if err != nil {
		return "", err
	}
	if solved {
		return ResultAlreadySolved, nil
	}

	challenge, err := s.challenges.GetEnabledByID(ctx, challengeID)
	if err != nil {
		return "", err
	}
	if challenge == nil {
		return "", ErrNotFound
	}

	isCorrect := challenge.Flag == flag
	if err := s.submissions.CreateSubmission(ctx, &model.Submission{
		UserID:        userID,
		ChallengeID:   challengeID,
		SubmittedFlag: flag,
		IsCorrect:     isCorrect,
	}); err != nil {
		return "", err
	}

	if isCorrect {
		event.Publish(event.Event{
			Type:          event.EventCorrectSubmission,
			UserID:        userID,
			ChallengeID:   challengeID,
			CompetitionID: nil,
			Ctx:           ctx,
		})
		return ResultCorrect, nil
	}
	return ResultIncorrect, nil
}

func (s *SubmissionService) List(ctx context.Context, userID string, challengeID string) ([]model.Submission, error) {
	return s.submissions.ListSubmissions(ctx, userID, challengeID)
}

func (s *SubmissionService) SubmitInComp(ctx context.Context, userID string, competitionID, challengeID string, flag string) (SubmitResult, error) {
	solved, err := s.submissions.HasCorrectSubmissionInComp(ctx, userID, challengeID, competitionID)
	if err != nil {
		return "", err
	}
	if solved {
		return ResultAlreadySolved, nil
	}

	challenge, err := s.challenges.GetEnabledByID(ctx, challengeID)
	if err != nil {
		return "", err
	}
	if challenge == nil {
		return "", ErrNotFound
	}

	isCorrect := challenge.Flag == flag
	compID := competitionID
	if err := s.submissions.CreateSubmissionWithComp(ctx, &model.Submission{
		UserID:        userID,
		ChallengeID:   challengeID,
		CompetitionID: &compID,
		SubmittedFlag: flag,
		IsCorrect:     isCorrect,
	}); err != nil {
		return "", err
	}

	if isCorrect {
		event.Publish(event.Event{
			Type:          event.EventCorrectSubmission,
			UserID:        userID,
			ChallengeID:   challengeID,
			CompetitionID: &compID,
			Ctx:           ctx,
		})
		return ResultCorrect, nil
	}
	return ResultIncorrect, nil
}

func (s *SubmissionService) ListByComp(ctx context.Context, competitionID string, userID string, challengeID string) ([]model.Submission, error) {
	return s.submissions.ListSubmissionsByComp(ctx, competitionID, userID, challengeID)
}
