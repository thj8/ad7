package store

import (
	"context"

	"ad7/internal/model"
)

type CompetitionStore interface {
	ListCompetitions(ctx context.Context) ([]model.Competition, error)
	ListActiveCompetitions(ctx context.Context) ([]model.Competition, error)
	GetCompetitionByID(ctx context.Context, resID int64) (*model.Competition, error)
	CreateCompetition(ctx context.Context, c *model.Competition) (int64, error)
	UpdateCompetition(ctx context.Context, c *model.Competition) error
	DeleteCompetition(ctx context.Context, resID int64) error
	AddChallenge(ctx context.Context, compID, chalID int64) error
	RemoveChallenge(ctx context.Context, compID, chalID int64) error
	ListCompChallenges(ctx context.Context, compID int64) ([]model.Challenge, error)
}
