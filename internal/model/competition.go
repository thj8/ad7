package model

type CompetitionMode string
type TeamJoinMode string

const (
	CompetitionModeIndividual CompetitionMode = "individual"
	CompetitionModeTeam       CompetitionMode = "team"

	TeamJoinModeFree    TeamJoinMode = "free"
	TeamJoinModeManaged TeamJoinMode = "managed"
)

// Competition 表示一个 CTF 比赛。
// 比赛有起止时间、激活状态，可以关联多道题目。
// 所有数据（排行榜、通知等）都限定在比赛范围内，没有全局概念。
type Competition struct {
	BaseModel
	Title         string          `json:"title"`       // 比赛标题
	Description   string          `json:"description"` // 比赛描述
	StartTime     Time            `json:"start_time"`  // 比赛开始时间
	EndTime       Time            `json:"end_time"`    // 比赛结束时间
	IsActive      bool            `json:"is_active"`   // 比赛是否激活
	Mode          CompetitionMode `json:"mode"`        // 比赛模式：个人或队伍
	TeamJoinMode  TeamJoinMode    `json:"team_join_mode"` // 队伍加入模式：自由或管理员管理
}

// CompetitionChallenge 表示比赛与题目的多对多关联关系。
// 一道题目可以被分配到多个比赛中，一个比赛可以包含多道题目。
type CompetitionChallenge struct {
	BaseModel
	CompetitionID string `json:"competition_id"`  // 比赛的 res_id
	ChallengeID   string `json:"challenge_id"`    // 题目的 res_id
}

// CompetitionTeam 表示比赛与队伍的多对多关联关系（仅用于管理员模式）。
type CompetitionTeam struct {
	BaseModel
	CompetitionID string `json:"competition_id"` // 比赛的 res_id
	TeamID        string `json:"team_id"`        // 队伍的 res_id
}

// Validate 验证 Competition 字段
func (c *Competition) Validate() error {
	if c.Title == "" {
		return ErrFieldRequired("title")
	}
	if len(c.Title) > 255 {
		return ErrFieldTooLong("title", 255)
	}
	if len(c.Description) > 4096 {
		return ErrFieldTooLong("description", 4096)
	}
	if c.StartTime.IsZero() {
		return ErrFieldRequired("start_time")
	}
	if c.EndTime.IsZero() {
		return ErrFieldRequired("end_time")
	}
	if c.EndTime.Time().Before(c.StartTime.Time()) {
		return ErrFieldInvalid("end_time", "must be after start_time")
	}
	if c.Mode != "" && c.Mode != CompetitionModeIndividual && c.Mode != CompetitionModeTeam {
		return ErrInvalidMode
	}
	if c.Mode == CompetitionModeTeam {
		if c.TeamJoinMode != "" && c.TeamJoinMode != TeamJoinModeFree && c.TeamJoinMode != TeamJoinModeManaged {
			return ErrFieldInvalid("team_join_mode", "must be free or managed")
		}
	}
	return nil
}

// Validate 验证 CompetitionChallenge 字段
func (cc *CompetitionChallenge) Validate() error {
	if cc.CompetitionID == "" {
		return ErrFieldRequired("competition_id")
	}
	if len(cc.CompetitionID) > 32 {
		return ErrFieldTooLong("competition_id", 32)
	}
	if cc.ChallengeID == "" {
		return ErrFieldRequired("challenge_id")
	}
	if len(cc.ChallengeID) > 32 {
		return ErrFieldTooLong("challenge_id", 32)
	}
	return nil
}

// Validate 验证 CompetitionTeam 字段
func (ct *CompetitionTeam) Validate() error {
	if ct.CompetitionID == "" {
		return ErrFieldRequired("competition_id")
	}
	if len(ct.CompetitionID) > 32 {
		return ErrFieldTooLong("competition_id", 32)
	}
	if ct.TeamID == "" {
		return ErrFieldRequired("team_id")
	}
	if len(ct.TeamID) > 32 {
		return ErrFieldTooLong("team_id", 32)
	}
	return nil
}
