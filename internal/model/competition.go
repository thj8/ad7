package model

import "time"

// Competition 表示一个 CTF 比赛。
// 比赛有起止时间、激活状态，可以关联多道题目。
// 所有数据（排行榜、通知等）都限定在比赛范围内，没有全局概念。
type Competition struct {
	BaseModel
	Title       string    `json:"title"`       // 比赛标题
	Description string    `json:"description"` // 比赛描述
	StartTime   time.Time `json:"start_time"`  // 比赛开始时间
	EndTime     time.Time `json:"end_time"`    // 比赛结束时间
	IsActive    bool      `json:"is_active"`   // 比赛是否激活
}

// CompetitionChallenge 表示比赛与题目的多对多关联关系。
// 一道题目可以被分配到多个比赛中，一个比赛可以包含多道题目。
type CompetitionChallenge struct {
	BaseModel
	CompetitionID string `json:"competition_id"`  // 比赛的 res_id
	ChallengeID   string `json:"challenge_id"`    // 题目的 res_id
}
