package dashboard

import (
	"context"
	"time"
)

func (p *Plugin) getCompetitionState(ctx context.Context, compID string) (*stateResponse, error) {
	resp := &stateResponse{}

	// 1. 获取比赛信息
	comp, err := p.getCompetitionInfo(ctx, compID)
	if err != nil {
		return nil, err
	}
	resp.Competition = *comp

	// 2. 获取题目列表及解题数
	challenges, err := p.getChallengeStates(ctx, compID)
	if err != nil {
		return nil, err
	}
	resp.Challenges = challenges

	// 3. 获取排行榜
	leaderboard, err := p.getLeaderboard(ctx, compID)
	if err != nil {
		return nil, err
	}
	resp.Leaderboard = leaderboard

	// 4. 获取统计
	stats, err := p.getStats(ctx, compID)
	if err != nil {
		return nil, err
	}
	resp.Stats = *stats

	// 5. 获取最近事件（过滤当前比赛）
	resp.RecentEvents = p.getRecentEventsForComp(compID)

	return resp, nil
}

func (p *Plugin) getCompetitionInfo(ctx context.Context, compID string) (*competitionInfo, error) {
	var info competitionInfo
	err := p.db.QueryRowContext(ctx, `
		SELECT res_id, title, is_active, start_time, end_time
		FROM competitions WHERE res_id = ? AND is_deleted = 0`, compID).
		Scan(&info.ID, &info.Title, &info.IsActive, &info.StartTime, &info.EndTime)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (p *Plugin) getChallengeStates(ctx context.Context, compID string) ([]challengeState, error) {
	// 先获取比赛关联的题目
	challengeRows, err := p.db.QueryContext(ctx, `
		SELECT c.res_id, c.title, c.category, c.score
		FROM competition_challenges cc
		JOIN challenges c ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ? AND c.is_enabled = 1 AND c.is_deleted = 0`, compID)
	if err != nil {
		return nil, err
	}
	defer challengeRows.Close()

	var challenges []challengeState
	challengeMap := make(map[string]*challengeState)
	for challengeRows.Next() {
		var cs challengeState
		if err := challengeRows.Scan(&cs.ID, &cs.Title, &cs.Category, &cs.Score); err != nil {
			return nil, err
		}
		challenges = append(challenges, cs)
		challengeMap[cs.ID] = &challenges[len(challenges)-1]
	}

	// 统计每道题的解题数
	solveRows, err := p.db.QueryContext(ctx, `
		SELECT s.challenge_id, COUNT(DISTINCT s.user_id)
		FROM submissions s
		WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0
		GROUP BY s.challenge_id`, compID)
	if err != nil {
		return nil, err
	}
	defer solveRows.Close()

	for solveRows.Next() {
		var chalID string
		var count int
		if err := solveRows.Scan(&chalID, &count); err != nil {
			return nil, err
		}
		if cs, ok := challengeMap[chalID]; ok {
			cs.SolveCount = count
		}
	}

	// 获取一血信息
	fbRows, err := p.db.QueryContext(ctx, `
		SELECT challenge_id, user_id, created_at
		FROM dashboard_first_blood WHERE competition_id = ?`, compID)
	if err != nil {
		return nil, err
	}
	defer fbRows.Close()

	for fbRows.Next() {
		var chalID string
		var userID string
		var createdAt time.Time
		if err := fbRows.Scan(&chalID, &userID, &createdAt); err != nil {
			return nil, err
		}
		if cs, ok := challengeMap[chalID]; ok {
			cs.FirstBlood = &firstBloodInfo{
				UserID:    userID,
				CreatedAt: createdAt,
			}
		}
	}

	return challenges, nil
}

func (p *Plugin) getLeaderboard(ctx context.Context, compID string) ([]leaderboardEntry, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT s.user_id, SUM(c.score), MAX(s.created_at)
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.is_correct = 1 AND s.competition_id = ? AND s.is_deleted = 0 AND c.is_deleted = 0
		GROUP BY s.user_id
		ORDER BY SUM(c.score) DESC, MAX(s.created_at) ASC`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var board []leaderboardEntry
	rank := 1
	for rows.Next() {
		var e leaderboardEntry
		if err := rows.Scan(&e.UserID, &e.TotalScore, &e.LastSolveAt); err != nil {
			return nil, err
		}
		e.Rank = rank
		rank++
		board = append(board, e)
	}
	if board == nil {
		board = []leaderboardEntry{}
	}
	return board, nil
}

func (p *Plugin) getStats(ctx context.Context, compID string) (*stats, error) {
	var s stats
	s.SolvesByCategory = make(map[string]int)

	// 总解题数
	err := p.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT CONCAT(user_id, '-', challenge_id))
		FROM submissions WHERE competition_id = ? AND is_correct = 1 AND is_deleted = 0`, compID).
		Scan(&s.TotalSolves)
	if err != nil {
		return nil, err
	}

	// 总参赛用户数
	err = p.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM submissions WHERE competition_id = ? AND is_deleted = 0`, compID).
		Scan(&s.TotalUsers)
	if err != nil {
		s.TotalUsers = 0
	}

	// 分类解题数
	rows, err := p.db.QueryContext(ctx, `
		SELECT c.category, COUNT(DISTINCT CONCAT(s.user_id, '-', s.challenge_id))
		FROM submissions s
		JOIN challenges c ON c.res_id = s.challenge_id
		WHERE s.competition_id = ? AND s.is_correct = 1 AND s.is_deleted = 0 AND c.is_deleted = 0
		GROUP BY c.category`, compID)
	if err != nil {
		return &s, nil
	}
	defer rows.Close()

	for rows.Next() {
		var cat string
		var count int
		if err := rows.Scan(&cat, &count); err != nil {
			continue
		}
		s.SolvesByCategory[cat] = count
	}

	return &s, nil
}

func (p *Plugin) getRecentEventsForComp(compID string) []recentEvent {
	allEvents := p.getRecentEvents()
	// 这里简化处理，实际可以更精确过滤
	return allEvents
}

func (p *Plugin) getFirstBloodList(ctx context.Context, compID string) ([]firstBlood, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT fb.res_id, fb.challenge_id, fb.user_id, fb.created_at,
		       c.title, c.category, c.score
		FROM dashboard_first_blood fb
		JOIN challenges c ON c.res_id = fb.challenge_id
		WHERE fb.competition_id = ? AND c.is_deleted = 0
		ORDER BY fb.created_at ASC`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []firstBlood
	for rows.Next() {
		var fb firstBlood
		if err := rows.Scan(&fb.ResID, &fb.ChallengeID, &fb.UserID, &fb.CreatedAt,
			&fb.ChallengeTitle, &fb.Category, &fb.Score); err != nil {
			return nil, err
		}
		list = append(list, fb)
	}
	if list == nil {
		list = []firstBlood{}
	}
	return list, nil
}
