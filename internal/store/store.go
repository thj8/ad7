package store

import (
	"context"

	"ad7/internal/model"
)

type ChallengeStore interface {
	ListEnabled(ctx context.Context) ([]model.Challenge, error)
	GetEnabledByID(ctx context.Context, resID string) (*model.Challenge, error)
	GetByID(ctx context.Context, resID string) (*model.Challenge, error)
	Create(ctx context.Context, c *model.Challenge) (string, error)
	Update(ctx context.Context, c *model.Challenge) error
	Delete(ctx context.Context, resID string) error
}

type SubmissionStore interface {
	HasCorrectSubmission(ctx context.Context, userID string, challengeID string) (bool, error)
	CreateSubmission(ctx context.Context, s *model.Submission) error
	ListSubmissions(ctx context.Context, userID string, challengeID string) ([]model.Submission, error)
	HasCorrectSubmissionInComp(ctx context.Context, userID string, challengeID, competitionID string) (bool, error)
	CreateSubmissionWithComp(ctx context.Context, s *model.Submission) error
	ListSubmissionsByComp(ctx context.Context, competitionID string, userID string, challengeID string) ([]model.Submission, error)
}

type CompetitionStore interface {
	ListCompetitions(ctx context.Context) ([]model.Competition, error)
	ListActiveCompetitions(ctx context.Context) ([]model.Competition, error)
	GetCompetitionByID(ctx context.Context, resID string) (*model.Competition, error)
	CreateCompetition(ctx context.Context, c *model.Competition) (string, error)
	UpdateCompetition(ctx context.Context, c *model.Competition) error
	DeleteCompetition(ctx context.Context, resID string) error
	AddChallenge(ctx context.Context, compID, chalID string) error
	RemoveChallenge(ctx context.Context, compID, chalID string) error
	ListCompChallenges(ctx context.Context, compID string) ([]model.Challenge, error)
}
