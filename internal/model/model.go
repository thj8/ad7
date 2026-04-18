package model

import "time"

type Challenge struct {
	ID          int       `json:"-"`
	ResID       string    `json:"id"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Score       int       `json:"score"`
	Flag        string    `json:"-"`
	IsEnabled   bool      `json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Submission struct {
	ID            int       `json:"-"`
	ResID         string    `json:"id"`
	UserID        string    `json:"user_id"`
	ChallengeID   string    `json:"challenge_id"`
	CompetitionID *string   `json:"competition_id"`
	SubmittedFlag string    `json:"submitted_flag"`
	IsCorrect     bool      `json:"is_correct"`
	CreatedAt     time.Time `json:"created_at"`
}

type Notification struct {
	ID            int       `json:"-"`
	ResID         string    `json:"id"`
	CompetitionID string    `json:"competition_id"`
	Title         string    `json:"title"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}
