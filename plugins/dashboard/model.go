package dashboard

import "time"

// recentEvent 表示仪表盘中的一个最近事件（解题或一血）。
// 保存在内存中用于实时展示。
type recentEvent struct {
	Type           string    `json:"type"`            // 事件类型："first_blood"（一血）或 "solve"（普通解题）
	UserID         string    `json:"user_id"`         // 解题用户 ID
	ChallengeID    string    `json:"challenge_id"`    // 题目 ID
	ChallengeTitle string    `json:"challenge_title"` // 题目标题
	Score          int       `json:"score,omitempty"` // 题目分值（仅普通解题事件）
	CreatedAt      time.Time `json:"created_at"`      // 事件发生时间
}

// firstBlood 表示一道题目的一血记录（首个正确提交者）。
type firstBlood struct {
	ResID          string    `json:"-"`               // 记录 ID，不暴露给 API
	CompetitionID  string    `json:"-"`               // 比赛 ID，不暴露给 API
	ChallengeID    string    `json:"challenge_id"`    // 题目 ID
	ChallengeTitle string    `json:"challenge_title"` // 题目标题
	Category       string    `json:"category"`        // 题目分类
	Score          int       `json:"score"`           // 题目分值
	UserID         string    `json:"user_id"`         // 一血获得者 ID
	CreatedAt      time.Time `json:"created_at"`      // 一血获得时间
}

// competitionInfo 表示比赛基本信息。
type competitionInfo struct {
	ID        string    `json:"id"`         // 比赛的 res_id
	Title     string    `json:"title"`      // 比赛标题
	IsActive  bool      `json:"is_active"`  // 是否激活
	StartTime time.Time `json:"start_time"` // 开始时间
	EndTime   time.Time `json:"end_time"`   // 结束时间
}

// challengeState 表示一道题目在比赛中的状态。
type challengeState struct {
	ID         string          `json:"id"`          // 题目的 res_id
	Title      string          `json:"title"`       // 题目标题
	Category   string          `json:"category"`    // 题目分类
	Score      int             `json:"score"`       // 题目分值
	SolveCount int             `json:"solve_count"` // 解题人数
	FirstBlood *firstBloodInfo `json:"first_blood"` // 一血信息（可能为空）
}

// firstBloodInfo 表示一血的简要信息（用于 challengeState 中）。
type firstBloodInfo struct {
	UserID    string    `json:"user_id"`    // 一血获得者 ID
	CreatedAt time.Time `json:"created_at"` // 一血获得时间
}

// leaderboardEntry 表示排行榜中的一条记录。
type leaderboardEntry struct {
	Rank        int       `json:"rank"`          // 排名
	UserID      string    `json:"user_id"`       // 用户 ID
	TotalScore  int       `json:"total_score"`   // 总得分
	LastSolveAt time.Time `json:"last_solve_at"` // 最后解题时间
}

// stats 表示比赛的统计数据。
type stats struct {
	TotalUsers        int            `json:"total_users"`         // 参赛总人数
	TotalSolves       int            `json:"total_solves"`        // 总解题数（去重）
	SolvesByCategory map[string]int `json:"solves_by_category"`  // 按分类的解题数
}

// stateResponse 是比赛状态总览的完整响应结构。
// 包含比赛信息、题目状态、排行榜、统计和最近事件。
type stateResponse struct {
	Competition  competitionInfo    `json:"competition"`   // 比赛基本信息
	Challenges   []challengeState   `json:"challenges"`    // 题目状态列表
	Leaderboard  []leaderboardEntry `json:"leaderboard"`   // 排行榜
	Stats        stats              `json:"stats"`         // 统计数据
	RecentEvents []recentEvent      `json:"recent_events"` // 最近事件
}
