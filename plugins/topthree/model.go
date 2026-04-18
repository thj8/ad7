package topthree

import "time"

type topThreeRecord struct {
	ID            int       `json:"-"`
	ResID         string    `json:"-"`
	CompetitionID string    `json:"-"`
	ChallengeID   string    `json:"-"`
	UserID        string    `json:"user_id"`
	Rank          int       `json:"rank"`
	CreatedAt     time.Time `json:"created_at"`
}

type challengeTopThree struct {
	ChallengeID string           `json:"challenge_id"`
	Title       string           `json:"title"`
	Category    string           `json:"category"`
	Score       int              `json:"score"`
	TopThree    []topThreeEntry  `json:"top_three"`
}

type topThreeEntry struct {
	Rank      int       `json:"rank"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type topThreeResponse struct {
	CompetitionID string              `json:"competition_id"`
	Challenges    []challengeTopThree `json:"challenges"`
}
