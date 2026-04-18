package model

import "time"

type Competition struct {
	BaseModel
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	IsActive    bool      `json:"is_active"`
}

type CompetitionChallenge struct {
	ID            int    `json:"-"`
	ResID         string `json:"id"`
	CompetitionID string `json:"competition_id"`
	ChallengeID   string `json:"challenge_id"`
}
