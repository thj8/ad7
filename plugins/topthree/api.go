package topthree

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (p *Plugin) getTopThree(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if len(compID) != 32 {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	chalRows, err := p.db.QueryContext(ctx, `
		SELECT c.res_id, c.title, c.category, c.score
		FROM challenges c
		INNER JOIN competition_challenges cc ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ? AND c.is_deleted = 0
	`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer chalRows.Close()

	var challenges []challengeTopThree
	for chalRows.Next() {
		var ct challengeTopThree
		err := chalRows.Scan(&ct.ChallengeID, &ct.Title, &ct.Category, &ct.Score)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		challenges = append(challenges, ct)
	}

	for i := range challenges {
		chal := &challenges[i]
		rows, err := p.db.QueryContext(ctx, `
			SELECT user_id, ranking, created_at
			FROM topthree_records
			WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 0
			ORDER BY ranking ASC
		`, compID, chal.ChallengeID)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		var topThree []topThreeEntry
		for rows.Next() {
			var e topThreeEntry
			err := rows.Scan(&e.UserID, &e.Rank, &e.CreatedAt)
			if err != nil {
				rows.Close()
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			topThree = append(topThree, e)
		}
		rows.Close()

		chal.TopThree = topThree
	}

	resp := topThreeResponse{
		CompetitionID: compID,
		Challenges:    challenges,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
