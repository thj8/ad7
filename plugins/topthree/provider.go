package topthree

import "context"

// TopThreeProvider 定义 topthree 插件暴露给其他插件的接口
type TopThreeProvider interface {
	// GetCompTopThree 获取比赛每道题目的三血信息
	// 返回值: map[challengeID]BloodRankEntry
	GetCompTopThree(ctx context.Context, compID string) (map[string]BloodRankEntry, error)
}

// BloodRankEntry 表示单道题目的三血排名信息
type BloodRankEntry struct {
	ChallengeID string
	FirstBlood  string // 用户ID
	SecondBlood string // 用户ID
	ThirdBlood  string // 用户ID
}
