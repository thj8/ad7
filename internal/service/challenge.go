package service

import (
	"context"
	"errors"

	"ad7/internal/model"
	"ad7/internal/store"
)

var ErrNotFound = errors.New("not found")

type ChallengeService struct {
	store store.ChallengeStore
}

func NewChallengeService(s store.ChallengeStore) *ChallengeService {
	return &ChallengeService{store: s}
}

func (s *ChallengeService) List(ctx context.Context) ([]model.Challenge, error) {
	return s.store.ListEnabled(ctx)
}

func (s *ChallengeService) Get(ctx context.Context, resID string) (*model.Challenge, error) {
	c, err := s.store.GetEnabledByID(ctx, resID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	return c, nil
}

func (s *ChallengeService) Create(ctx context.Context, c *model.Challenge) (string, error) {
	if c.Title == "" || c.Flag == "" {
		return "", errors.New("title and flag are required")
	}
	if c.Score <= 0 {
		c.Score = 100
	}
	if c.Category == "" {
		c.Category = "misc"
	}
	c.IsEnabled = true
	return s.store.Create(ctx, c)
}

func (s *ChallengeService) Update(ctx context.Context, resID string, patch *model.Challenge) error {
	existing, err := s.store.GetByID(ctx, resID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	if patch.Title != "" {
		existing.Title = patch.Title
	}
	if patch.Category != "" {
		existing.Category = patch.Category
	}
	if patch.Description != "" {
		existing.Description = patch.Description
	}
	if patch.Score > 0 {
		existing.Score = patch.Score
	}
	if patch.Flag != "" {
		existing.Flag = patch.Flag
	}
	existing.IsEnabled = patch.IsEnabled // PUT always sets is_enabled explicitly
	return s.store.Update(ctx, existing)
}

func (s *ChallengeService) Delete(ctx context.Context, resID string) error {
	return s.store.Delete(ctx, resID)
}
