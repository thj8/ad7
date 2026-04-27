package topthree

import (
	"time"

	"ad7/internal/model"
)

// topThreeRecord 是三血记录的数据库模型。
// 记录了每道题目的前三名正确提交者。
// 继承 BaseModel 以支持软删除和 UUID 标识。
type topThreeRecord struct {
	model.BaseModel
	CompetitionID string `json:"-"`         // 所属比赛 ID，不暴露给 API
	ChallengeID   string `json:"-"`         // 题目 ID，不暴露给 API
	UserID        string `json:"user_id"`   // 三血获得者 ID
	TeamID        string `json:"team_id"`   // 三血获得者队伍 ID（如果是队伍模式）
	Ranking       int    `json:"ranking"`   // 排名（1=一血，2=二血，3=三血）
}

// challengeTopThree 表示一道题目及其三血排名信息。
type challengeTopThree struct {
	ChallengeID string          `json:"challenge_id"` // 题目 ID
	Title       string          `json:"title"`        // 题目标题
	Category    string          `json:"category"`     // 题目分类
	Score       int             `json:"score"`        // 题目分值
	TopThree    []topThreeEntry `json:"top_three"`    // 三血排名列表
}

// topThreeEntry 表示一个三血排名条目。
type topThreeEntry struct {
	Ranking   int       `json:"ranking"`    // 排名（1/2/3）
	UserID    string    `json:"user_id"`    // 获得者 ID
	TeamID    string    `json:"team_id"`    // 获得者队伍 ID（如果是队伍模式）
	CreatedAt time.Time `json:"created_at"` // 获得时间
}

// topThreeResponse 是三血排名接口的响应结构。
type topThreeResponse struct {
	CompetitionID string              `json:"competition_id"` // 比赛 ID
	Challenges    []challengeTopThree `json:"challenges"`     // 各题目的三血排名
}
