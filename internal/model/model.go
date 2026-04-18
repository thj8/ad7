package model

import "time"

// BaseModel contains common fields for all entities with soft delete support
type BaseModel struct {
	ID        int       `json:"-"`
	ResID     string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsDeleted bool      `json:"-"`
}

type Challenge struct {
	BaseModel
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Score       int    `json:"score"`
	Flag        string `json:"-"`
	IsEnabled   bool   `json:"is_enabled"`
}

type Submission struct {
	BaseModel
	UserID        string    `json:"user_id"`
	ChallengeID   string    `json:"challenge_id"`
	CompetitionID *string   `json:"competition_id"`
	SubmittedFlag string    `json:"submitted_flag"`
	IsCorrect     bool      `json:"is_correct"`
}

type Notification struct {
	BaseModel
	CompetitionID string `json:"competition_id"`
	Title         string `json:"title"`
	Message       string `json:"message"`
}
