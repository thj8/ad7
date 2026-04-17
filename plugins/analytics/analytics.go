package analytics

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

type Plugin struct{ db *sql.DB }

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/overview", p.overview)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/categories", p.byCategory)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/users", p.userStats)
	r.With(auth.Authenticate).Get("/api/v1/competitions/{id}/analytics/challenges", p.challengeStats)
}

type overviewResponse struct {
	TotalUsers         int     `json:"total_users"`
	TotalChallenges    int     `json:"total_challenges"`
	TotalSubmissions   int     `json:"total_submissions"`
	CorrectSubmissions int     `json:"correct_submissions"`
	AverageSolves      float64 `json:"average_solves"`
	AverageSolveTime   string  `json:"average_solve_time_seconds"`
	CompletionRate     float64 `json:"completion_rate"`
}

type categoryStats struct {
	Category         string  `json:"category"`
	TotalChallenges  int     `json:"total_challenges"`
	TotalSolves      int     `json:"total_solves"`
	UniqueUsersSolved int    `json:"unique_users_solved"`
	AverageSolves    float64 `json:"average_solves_per_user"`
	SuccessRate      float64 `json:"success_rate"`
}

type categoryResponse struct {
	Categories []categoryStats `json:"categories"`
}

func (p *Plugin) overview(w http.ResponseWriter, r *http.Request) {
	compID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var resp overviewResponse

	// Get total challenges in competition
	err = p.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM competition_challenges WHERE competition_id = ?
	`, compID).Scan(&resp.TotalChallenges)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	// Get total users who submitted
	err = p.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM submissions WHERE competition_id = ?
	`, compID).Scan(&resp.TotalUsers)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	// Get total submissions and correct submissions
	err = p.db.QueryRowContext(ctx, `
		SELECT COUNT(*), SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END)
		FROM submissions WHERE competition_id = ?
	`, compID).Scan(&resp.TotalSubmissions, &resp.CorrectSubmissions)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	// Calculate average solves per user (users who have at least one correct submission)
	if resp.TotalUsers > 0 {
		var totalCorrectSolves int
		err = p.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM submissions
			WHERE competition_id = ? AND is_correct = 1
		`, compID).Scan(&totalCorrectSolves)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		resp.AverageSolves = float64(totalCorrectSolves) / float64(resp.TotalUsers)

		// Calculate completion rate (average % of challenges solved per user)
		if resp.TotalChallenges > 0 {
			resp.CompletionRate = (resp.AverageSolves / float64(resp.TotalChallenges)) * 100
		}
	}

	// Calculate average time to first solve (for challenges that have been solved)
	// This is the average time between competition start and first correct submission
	var avgSolveTimeSec sql.NullFloat64
	err = p.db.QueryRowContext(ctx, `
		SELECT AVG(TIMESTAMPDIFF(SECOND, c.start_time, s.created_at))
		FROM submissions s
		JOIN competitions c ON c.res_id = s.competition_id
		WHERE s.competition_id = ? AND s.is_correct = 1
	`, compID).Scan(&avgSolveTimeSec)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if avgSolveTimeSec.Valid {
		resp.AverageSolveTime = strconv.FormatFloat(avgSolveTimeSec.Float64, 'f', 2, 64)
	} else {
		resp.AverageSolveTime = "0"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (p *Plugin) byCategory(w http.ResponseWriter, r *http.Request) {
	compID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get all challenges in this competition with their categories
	rows, err := p.db.QueryContext(ctx, `
		SELECT c.category, COUNT(DISTINCT cc.challenge_id) as total_challenges
		FROM competition_challenges cc
		JOIN challenges c ON c.res_id = cc.challenge_id
		WHERE cc.competition_id = ?
		GROUP BY c.category
	`, compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var categories []categoryStats
	for rows.Next() {
		var cat categoryStats
		if err := rows.Scan(&cat.Category, &cat.TotalChallenges); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		// Get total correct solves for this category
		err = p.db.QueryRowContext(ctx, `
			SELECT COUNT(*), COUNT(DISTINCT s.user_id)
			FROM submissions s
			JOIN challenges c ON c.res_id = s.challenge_id
			JOIN competition_challenges cc ON cc.challenge_id = c.res_id
			WHERE s.competition_id = ? AND cc.competition_id = ?
			AND s.is_correct = 1 AND c.category = ?
		`, compID, compID, cat.Category).Scan(&cat.TotalSolves, &cat.UniqueUsersSolved)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		// Get total users in competition
		var totalUsers int
		err = p.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT user_id) FROM submissions WHERE competition_id = ?
		`, compID).Scan(&totalUsers)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		if totalUsers > 0 {
			cat.AverageSolves = float64(cat.TotalSolves) / float64(totalUsers)
		}

		// Get total attempts (all submissions) for this category
		var totalAttempts int
		err = p.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM submissions s
			JOIN challenges c ON c.res_id = s.challenge_id
			JOIN competition_challenges cc ON cc.challenge_id = c.res_id
			WHERE s.competition_id = ? AND cc.competition_id = ?
			AND c.category = ?
		`, compID, compID, cat.Category).Scan(&totalAttempts)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		if totalAttempts > 0 {
			cat.SuccessRate = (float64(cat.TotalSolves) / float64(totalAttempts)) * 100
		}

		categories = append(categories, cat)
	}

	if categories == nil {
		categories = []categoryStats{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categoryResponse{Categories: categories})
}

func (p *Plugin) userStats(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (p *Plugin) challengeStats(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}
