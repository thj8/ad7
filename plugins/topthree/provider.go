package topthree

import "context"

// TopThreeProvider 定义 topthree 插件暴露给其他插件的接口
type TopThreeProvider interface {
	// GetBloodRank 获取用户在某道题目的三血排名
	// 返回值: 1=一血, 2=二血, 3=三血, 0=未入榜
	GetBloodRank(ctx context.Context, compID, chalID, userID string) (int, error)

	// GetCompTopThree 获取比赛每道题目的三血信息
	// 返回值: map[challengeID]BloodRankEntry
	GetCompTopThree(ctx context.Context, compID string) (map[string]BloodRankEntry, error)

	// IsTopThreeFull 检查某道题目的 top3 是否已填满（3项）
	IsTopThreeFull(ctx context.Context, compID, chalID string) bool
}

// BloodRankEntry 表示单道题目的三血排名信息
type BloodRankEntry struct {
	ChallengeID string
	FirstBlood  string // 用户ID
	SecondBlood string // 用户ID
	ThirdBlood  string // 用户ID
}
