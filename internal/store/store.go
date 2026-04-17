package store

import (
	"context"

	"ad7/internal/model"
)

type ChallengeStore interface {
	ListEnabled(ctx context.Context) ([]model.Challenge, error)
	GetEnabledByID(ctx context.Context, resID int64) (*model.Challenge, error)
	GetByID(ctx context.Context, resID int64) (*model.Challenge, error)
	Create(ctx context.Context, c *model.Challenge) (int64, error)
	Update(ctx context.Context, c *model.Challenge) error
	Delete(ctx context.Context, resID int64) error
}

type SubmissionStore interface {
	HasCorrectSubmission(ctx context.Context, userID string, challengeID int64) (bool, error)
	CreateSubmission(ctx context.Context, s *model.Submission) error
	ListSubmissions(ctx context.Context, userID string, challengeID int64) ([]model.Submission, error)
	HasCorrectSubmissionInComp(ctx context.Context, userID string, challengeID, competitionID int64) (bool, error)
	CreateSubmissionWithComp(ctx context.Context, s *model.Submission) error
	ListSubmissionsByComp(ctx context.Context, competitionID int64, userID string, challengeID int64) ([]model.Submission, error)
}
