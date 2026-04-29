package topthree

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ad7/internal/pluginutil"
)

// getTopThree 处理获取比赛三血排名的请求。
// 返回指定比赛中每道题目的前三名正确提交者信息。
func (p *Plugin) getTopThree(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if err := pluginutil.ParseID(compID); err != nil {
		pluginutil.WriteError(w, http.StatusBadRequest, "invalid competition id")
		return
	}

	cached, err := pluginutil.WithCache(p.cache, "topthree:"+compID, func() (any, error) {
		return p.getTopThreeFromDB(r.Context(), compID)
	})

	if err != nil {
		pluginutil.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}

	pluginutil.WriteJSON(w, http.StatusOK, cached.(topThreeResponse))
}

// getTopThreeFromDB 从数据库获取比赛三血排名数据
func (p *Plugin) getTopThreeFromDB(ctx context.Context, compID string) (topThreeResponse, error) {
	// 查询比赛中所有题目
	chalRows, err := p.db.QueryContext(ctx, `
		SELECT c.res_id, c.title, c.category, c.score
		FROM challenges c
		INNER JOIN competition_challenges cc ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ? AND c.is_deleted = 0 AND cc.is_deleted = 0
	`, compID)
	if err != nil {
		return topThreeResponse{}, err
	}
	defer chalRows.Close()

	// 收集题目信息
	challengeMap := make(map[string]*challengeTopThree)
	var chalOrder []string
	for chalRows.Next() {
		var ct challengeTopThree
		if err := chalRows.Scan(&ct.ChallengeID, &ct.Title, &ct.Category, &ct.Score); err != nil {
			return topThreeResponse{}, err
		}
		challengeMap[ct.ChallengeID] = &ct
		chalOrder = append(chalOrder, ct.ChallengeID)
	}

	// 单次查询获取该比赛所有三血记录（消除 N+1 问题）
	rows, err := p.db.QueryContext(ctx, `
		SELECT challenge_id, user_id, team_id, ranking, created_at
		FROM topthree_records
		WHERE competition_id = ? AND is_deleted = 0
		ORDER BY ranking ASC
	`, compID)
	if err != nil {
		return topThreeResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var chalID string
		var e topThreeEntry
		var teamID sql.NullString
		if err := rows.Scan(&chalID, &e.UserID, &teamID, &e.Ranking, &e.CreatedAt); err != nil {
			return topThreeResponse{}, err
		}
		if teamID.Valid {
			e.TeamID = teamID.String
		}
		if ct, ok := challengeMap[chalID]; ok {
			ct.TopThree = append(ct.TopThree, e)
		}
	}

	// 按题目顺序构建响应
	challenges := make([]challengeTopThree, 0, len(chalOrder))
	for _, id := range chalOrder {
		challenges = append(challenges, *challengeMap[id])
	}

	return topThreeResponse{
		CompetitionID: compID,
		Challenges:    challenges,
	}, nil
}
