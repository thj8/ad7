package model

import "time"

type Challenge struct {
	ID          int       `json:"-"`
	ResID       int64     `json:"id"`
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
	ResID         int64     `json:"id"`
	UserID        string    `json:"user_id"`
	ChallengeID   int64     `json:"challenge_id"`
	CompetitionID *int64    `json:"competition_id"`
	SubmittedFlag string    `json:"submitted_flag"`
	IsCorrect     bool      `json:"is_correct"`
	CreatedAt     time.Time `json:"created_at"`
}

type Notification struct {
	ID            int       `json:"-"`
	ResID         int64     `json:"id"`
	CompetitionID *int64    `json:"competition_id"`
	ChallengeID   *int64    `json:"challenge_id"`
	Title         string    `json:"title"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}
