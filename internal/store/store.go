package store

import (
	"context"

	"ad7/internal/model"
)

type ChallengeStore interface {
	ListEnabled(ctx context.Context) ([]model.Challenge, error)
	GetEnabledByID(ctx context.Context, id int) (*model.Challenge, error)
	GetByID(ctx context.Context, id int) (*model.Challenge, error)
	Create(ctx context.Context, c *model.Challenge) (int64, error)
	Update(ctx context.Context, c *model.Challenge) error
	Delete(ctx context.Context, id int) error
}

type SubmissionStore interface {
	HasCorrectSubmission(ctx context.Context, userID string, challengeID int) (bool, error)
	CreateSubmission(ctx context.Context, s *model.Submission) error
	ListSubmissions(ctx context.Context, userID string, challengeID int) ([]model.Submission, error)
}
