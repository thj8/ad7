package dashboard

import "time"

type recentEvent struct {
	Type           string    `json:"type"`
	UserID         string    `json:"user_id"`
	ChallengeID    string    `json:"challenge_id"`
	ChallengeTitle string    `json:"challenge_title"`
	Score          int       `json:"score,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type firstBlood struct {
	ResID         string    `json:"-"`
	CompetitionID string    `json:"-"`
	ChallengeID   string    `json:"challenge_id"`
	ChallengeTitle string   `json:"challenge_title"`
	Category      string    `json:"category"`
	Score         int       `json:"score"`
	UserID        string    `json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type competitionInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	IsActive  bool      `json:"is_active"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

type challengeState struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Category   string          `json:"category"`
	Score      int             `json:"score"`
	SolveCount int             `json:"solve_count"`
	FirstBlood *firstBloodInfo `json:"first_blood"`
}

type firstBloodInfo struct {
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type leaderboardEntry struct {
	Rank         int       `json:"rank"`
	UserID       string    `json:"user_id"`
	TotalScore   int       `json:"total_score"`
	LastSolveAt  time.Time `json:"last_solve_at"`
}

type stats struct {
	TotalUsers        int            `json:"total_users"`
	TotalSolves       int            `json:"total_solves"`
	SolvesByCategory map[string]int `json:"solves_by_category"`
}

type stateResponse struct {
	Competition  competitionInfo    `json:"competition"`
	Challenges   []challengeState   `json:"challenges"`
	Leaderboard  []leaderboardEntry `json:"leaderboard"`
	Stats        stats              `json:"stats"`
	RecentEvents []recentEvent      `json:"recent_events"`
}
