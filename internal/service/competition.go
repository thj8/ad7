package service

import (
	"context"
	"errors"

	"ad7/internal/model"
	"ad7/internal/store"
)

type CompetitionService struct {
	store store.CompetitionStore
}

func NewCompetitionService(s store.CompetitionStore) *CompetitionService {
	return &CompetitionService{store: s}
}

func (s *CompetitionService) List(ctx context.Context) ([]model.Competition, error) {
	return s.store.ListCompetitions(ctx)
}

func (s *CompetitionService) ListActive(ctx context.Context) ([]model.Competition, error) {
	return s.store.ListActiveCompetitions(ctx)
}

func (s *CompetitionService) Get(ctx context.Context, resID int64) (*model.Competition, error) {
	c, err := s.store.GetCompetitionByID(ctx, resID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	return c, nil
}

func (s *CompetitionService) Create(ctx context.Context, c *model.Competition) (int64, error) {
	if c.Title == "" {
		return 0, errors.New("title is required")
	}
	if c.EndTime.Before(c.StartTime) {
		return 0, errors.New("end_time must be after start_time")
	}
	return s.store.CreateCompetition(ctx, c)
}

func (s *CompetitionService) Update(ctx context.Context, resID int64, patch *model.Competition) error {
	existing, err := s.store.GetCompetitionByID(ctx, resID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	if patch.Title != "" {
		existing.Title = patch.Title
	}
	if patch.Description != "" {
		existing.Description = patch.Description
	}
	if !patch.StartTime.IsZero() {
		existing.StartTime = patch.StartTime
	}
	if !patch.EndTime.IsZero() {
		existing.EndTime = patch.EndTime
	}
	if existing.EndTime.Before(existing.StartTime) {
		return errors.New("end_time must be after start_time")
	}
	existing.IsActive = patch.IsActive
	return s.store.UpdateCompetition(ctx, existing)
}

func (s *CompetitionService) Delete(ctx context.Context, resID int64) error {
	return s.store.DeleteCompetition(ctx, resID)
}

func (s *CompetitionService) AddChallenge(ctx context.Context, compID, chalID int64) error {
	return s.store.AddChallenge(ctx, compID, chalID)
}

func (s *CompetitionService) RemoveChallenge(ctx context.Context, compID, chalID int64) error {
	return s.store.RemoveChallenge(ctx, compID, chalID)
}

func (s *CompetitionService) ListChallenges(ctx context.Context, compID int64) ([]model.Challenge, error) {
	return s.store.ListCompChallenges(ctx, compID)
}
