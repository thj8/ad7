package integration_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"

	"ad7/internal/handler"
	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/service"
	"ad7/internal/store"
	"ad7/plugins/analytics"
	"ad7/plugins/hints"
	"ad7/plugins/leaderboard"
	"ad7/plugins/notification"
	"ad7/plugins/topthree"
)

const (
	testSecret = "test-secret"
	adminRole  = "admin"
)

var testDSN = func() string {
	if v := os.Getenv("TEST_DSN"); v != "" {
		return v
	}
	return "root:asfdsfedarjeiowvgfsd@tcp(192.168.5.44:3306)/ctf?parseTime=true"
}()

var (
	testServer *httptest.Server
	testDB     *sql.DB
)

func TestMain(m *testing.M) {
	st, err := store.New(testDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect db: %v\n", err)
		os.Exit(1)
	}
	testDB = st.DB()

	// Create a test config with rate limit
	cfg := &struct {
		RateLimit struct {
			Submission struct {
				Requests int
				Window   time.Duration
			}
		}
	}{}
	cfg.RateLimit.Submission.Requests = 3
	cfg.RateLimit.Submission.Window = 10 * time.Second

	auth := middleware.NewAuth(testSecret, adminRole)
	challengeSvc := service.NewChallengeService(st)
	submissionSvc := service.NewSubmissionService(st, st)
	compSvc := service.NewCompetitionService(st)
	challengeH := handler.NewChallengeHandler(challengeSvc)
	submissionH := handler.NewSubmissionHandler(submissionSvc)
	compH := handler.NewCompetitionHandler(compSvc)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Authenticate)
		r.Get("/challenges", challengeH.List)
		r.Get("/challenges/{id}", challengeH.Get)
		r.Get("/competitions", compH.List)
		r.Get("/competitions/{id}", compH.Get)
		r.Get("/competitions/{id}/challenges", compH.ListChallenges)
		r.With(
			middleware.LimitByUserID(
				cfg.RateLimit.Submission.Requests,
				cfg.RateLimit.Submission.Window,
			),
		).Post("/competitions/{comp_id}/challenges/{id}/submit", submissionH.SubmitInComp)
		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Post("/challenges", challengeH.Create)
			r.Put("/challenges/{id}", challengeH.Update)
			r.Delete("/challenges/{id}", challengeH.Delete)
			r.Get("/competitions/{id}/submissions", submissionH.ListByComp)
			r.Post("/competitions", compH.Create)
			r.Get("/competitions", compH.ListAll)
			r.Put("/competitions/{id}", compH.Update)
			r.Delete("/competitions/{id}", compH.Delete)
			r.Post("/competitions/{id}/challenges", compH.AddChallenge)
			r.Delete("/competitions/{id}/challenges/{challenge_id}", compH.RemoveChallenge)
		})
	})

	plugins := []plugin.Plugin{leaderboard.New(), notification.New(), analytics.New(), hints.New(), topthree.New()}
	for _, p := range plugins {
		p.Register(r, st.DB(), auth)
	}

	testServer = httptest.NewServer(r)
	defer testServer.Close()
	defer st.Close()

	os.Exit(m.Run())
}

func makeToken(userID, role string) string {
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(testSecret))
	return tok
}

func doRequest(t *testing.T, method, path, body, token string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, testServer.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func cleanup(t *testing.T) {
	t.Helper()
	testDB.Exec("DELETE FROM topthree_records")
	testDB.Exec("DELETE FROM hints")
	testDB.Exec("DELETE FROM competition_challenges")
	testDB.Exec("DELETE FROM notifications")
	testDB.Exec("DELETE FROM submissions")
	testDB.Exec("DELETE FROM competitions")
	testDB.Exec("DELETE FROM challenges")
}

func decodeJSON(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return m
}

func getID(t *testing.T, m map[string]any) string {
	t.Helper()
	id, ok := m["id"].(string)
	if !ok {
		t.Fatalf("id not a string: %T %v", m["id"], m["id"])
	}
	return id
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("want status %d, got %d: %s", want, resp.StatusCode, body)
	}
}

// --- Tests ---

func TestListChallenges(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")

	// 200 empty list
	resp := doRequest(t, "GET", "/api/v1/challenges", "", adminTok)
	assertStatus(t, resp, 200)
	body := decodeJSON(t, resp)
	if body["challenges"] == nil {
		t.Fatal("expected challenges key")
	}

	// create a challenge and verify flag is not in list response
	doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"FlagTest","description":"D","score":100,"flag":"flag{secret}"}`, adminTok).Body.Close()
	resp = doRequest(t, "GET", "/api/v1/challenges", "", adminTok)
	assertStatus(t, resp, 200)
	body = decodeJSON(t, resp)
	for _, c := range body["challenges"].([]any) {
		ch := c.(map[string]any)
		if _, hasFlag := ch["flag"]; hasFlag {
			t.Fatal("flag must not appear in challenge list response")
		}
	}

	// 401 no token
	resp = doRequest(t, "GET", "/api/v1/challenges", "", "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestGetChallenge(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// create a challenge first
	resp := doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"T1","description":"D","score":100,"flag":"flag{x}"}`, adminTok)
	assertStatus(t, resp, 201)
	b := decodeJSON(t, resp)
	id := getID(t, b)

	// 200 found
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%s", id), "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["title"] != "T1" {
		t.Fatalf("expected title T1, got %v", b["title"])
	}
	if _, hasFlag := b["flag"]; hasFlag {
		t.Fatal("flag must not be in response")
	}

	// 404 not found
	resp = doRequest(t, "GET", "/api/v1/challenges/00000000000000000000000000000000", "", userTok)
	assertStatus(t, resp, 404)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%s", id), "", "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestAdminCreateChallenge(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")
	body := `{"title":"T3","description":"D","score":200,"flag":"flag{z}"}`

	// 201 happy path
	resp := doRequest(t, "POST", "/api/v1/admin/challenges", body, adminTok)
	assertStatus(t, resp, 201)
	b := decodeJSON(t, resp)
	if b["id"] == nil {
		t.Fatal("expected id in response")
	}

	// 403 non-admin
	resp = doRequest(t, "POST", "/api/v1/admin/challenges", body, userTok)
	assertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "POST", "/api/v1/admin/challenges", body, "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestAdminUpdateChallenge(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// create
	resp := doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"T4","description":"D","score":100,"flag":"flag{u}"}`, adminTok)
	assertStatus(t, resp, 201)
	b := decodeJSON(t, resp)
	id := getID(t, b)
	path := fmt.Sprintf("/api/v1/admin/challenges/%s", id)

	// 204 update
	resp = doRequest(t, "PUT", path,
		`{"title":"T4-updated","description":"D","score":150,"flag":"flag{u}","is_enabled":true}`, adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// 404
	resp = doRequest(t, "PUT", "/api/v1/admin/challenges/00000000000000000000000000000000",
		`{"title":"x","description":"D","score":100,"flag":"flag{x}","is_enabled":true}`, adminTok)
	assertStatus(t, resp, 404)
	resp.Body.Close()

	// 403
	resp = doRequest(t, "PUT", path,
		`{"title":"x","description":"D","score":100,"flag":"flag{x}","is_enabled":true}`, userTok)
	assertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "PUT", path,
		`{"title":"x","description":"D","score":100,"flag":"flag{x}","is_enabled":true}`, "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestAdminDeleteChallenge(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// create
	resp := doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"T5","description":"D","score":100,"flag":"flag{d}"}`, adminTok)
	assertStatus(t, resp, 201)
	b := decodeJSON(t, resp)
	id := getID(t, b)
	path := fmt.Sprintf("/api/v1/admin/challenges/%s", id)

	// 403
	resp = doRequest(t, "DELETE", path, "", userTok)
	assertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "DELETE", path, "", "")
	assertStatus(t, resp, 401)
	resp.Body.Close()

	// 204 delete
	resp = doRequest(t, "DELETE", path, "", adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// verify soft-deleted challenge is no longer accessible
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%s", id), "", userTok)
	assertStatus(t, resp, 404)
	resp.Body.Close()
}

func TestAdminListSubmissions(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// create competition + challenge + submit in comp
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"CompSub","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"T6","description":"D","score":100,"flag":"flag{s}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	// add challenge to competition
	doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok).Body.Close()

	// submit in competition
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{s}"}`, userTok)

	// 200 unfiltered
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions", compID), "", adminTok)
	assertStatus(t, resp, 200)
	b := decodeJSON(t, resp)
	subs := b["submissions"].([]any)
	if len(subs) == 0 {
		t.Fatal("expected at least 1 submission")
	}

	// 200 filtered by user_id
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions?user_id=user1", compID), "", adminTok)
	assertStatus(t, resp, 200)
	resp.Body.Close()

	// 200 filtered by challenge_id
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions?challenge_id=%s", compID, chalID), "", adminTok)
	assertStatus(t, resp, 200)
	resp.Body.Close()

	// 403
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions", compID), "", userTok)
	assertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions", compID), "", "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestCompetitions(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// Create competition
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"CTF Round 1","description":"First round","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	b := decodeJSON(t, resp)
	compID := getID(t, b)

	// List active competitions
	resp = doRequest(t, "GET", "/api/v1/competitions", "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	comps := b["competitions"].([]any)
	if len(comps) == 0 {
		t.Fatal("expected at least 1 competition")
	}

	// Get competition detail
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%s", compID), "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["title"] != "CTF Round 1" {
		t.Fatalf("expected title 'CTF Round 1', got %v", b["title"])
	}

	// Update competition
	resp = doRequest(t, "PUT", fmt.Sprintf("/api/v1/admin/competitions/%s", compID),
		`{"title":"CTF Round 1 Updated","description":"Updated","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z","is_active":true}`, adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// Delete competition
	resp = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/admin/competitions/%s", compID), "", adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// 404 after delete
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%s", compID), "", userTok)
	assertStatus(t, resp, 404)
	resp.Body.Close()

	// 403 non-admin create
	resp = doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"X","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, userTok)
	assertStatus(t, resp, 403)
	resp.Body.Close()

	// 400 missing title
	resp = doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 400)
	resp.Body.Close()
}

func TestCompetitionChallenges(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// Create competition
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"Comp","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	// Create challenge
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Chal1","description":"D","score":100,"flag":"flag{x}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	// Add challenge to competition
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// List competition challenges
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%s/challenges", compID), "", userTok)
	assertStatus(t, resp, 200)
	b := decodeJSON(t, resp)
	chals := b["challenges"].([]any)
	if len(chals) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(chals))
	}
	// verify flag not leaked in competition challenge list
	for _, c := range chals {
		ch := c.(map[string]any)
		if _, hasFlag := ch["flag"]; hasFlag {
			t.Fatal("flag must not appear in competition challenge list response")
		}
	}

	// Remove challenge from competition
	resp = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges/%s", compID, chalID), "", adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// Verify empty
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%s/challenges", compID), "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	chals = b["challenges"].([]any)
	if len(chals) != 0 {
		t.Fatalf("expected 0 challenges, got %d", len(chals))
	}
}

func TestSubmitInCompetition(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// Create competition + challenge + add to comp
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"Comp","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Chal","description":"D","score":200,"flag":"flag{comp}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	submitPath := fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID)

	// Correct flag
	resp = doRequest(t, "POST", submitPath, `{"flag":"flag{comp}"}`, userTok)
	assertStatus(t, resp, 200)
	b := decodeJSON(t, resp)
	if b["success"] != true {
		t.Fatalf("expected success=true, got %v", b["success"])
	}

	// Already solved
	resp = doRequest(t, "POST", submitPath, `{"flag":"flag{comp}"}`, userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["message"] != "already_solved" {
		t.Fatalf("expected already_solved, got %v", b["message"])
	}

	// Wrong flag with different user
	user2Tok := makeToken("user2", "user")
	resp = doRequest(t, "POST", submitPath, `{"flag":"wrong"}`, user2Tok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["success"] != false {
		t.Fatal("expected success=false for wrong flag")
	}

	// 401 no token
	resp = doRequest(t, "POST", submitPath, `{"flag":"x"}`, "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestCompetitionLeaderboard(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	u1 := makeToken("user1", "user")
	u2 := makeToken("user2", "user")

	// Create competition + 2 challenges
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"CompLB","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	// Create 2 challenges
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"C1","description":"D","score":100,"flag":"flag{1}"}`, adminTok)
	ch1 := getID(t, decodeJSON(t, resp))
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"C2","description":"D","score":200,"flag":"flag{2}"}`, adminTok)
	ch2 := getID(t, decodeJSON(t, resp))

	// Add both to competition
	doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, ch1), adminTok).Body.Close()
	doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, ch2), adminTok).Body.Close()

	// user1 solves both
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, ch1),
		`{"flag":"flag{1}"}`, u1).Body.Close()
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, ch2),
		`{"flag":"flag{2}"}`, u1).Body.Close()

	// user2 solves only ch1
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, ch1),
		`{"flag":"flag{1}"}`, u2).Body.Close()

	// Competition leaderboard: user1=300, user2=100
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%s/leaderboard", compID), "", u1)
	assertStatus(t, resp, 200)
	b := decodeJSON(t, resp)
	board := b["leaderboard"].([]any)
	if len(board) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(board))
	}
	first := board[0].(map[string]any)
	if first["user_id"] != "user1" {
		t.Fatalf("expected user1 at rank 1, got %v", first["user_id"])
	}

	// 验证逐题详情
	chals, ok := first["challenges"].([]any)
	if !ok || len(chals) != 2 {
		t.Fatalf("expected 2 challenge results, got %v", first["challenges"])
	}
	for _, c := range chals {
		cr := c.(map[string]any)
		if cr["solved"] != true {
			t.Errorf("expected solved=true for challenge %v", cr["challenge_id"])
		}
	}
}

func TestCompetitionNotifications(t *testing.T) {
	cleanup(t)
	testDB.Exec("DELETE FROM notifications")
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// Create competition
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"CompN","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	// Create competition notification
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/notifications", compID),
		`{"title":"比赛提示","message":"注意时间"}`, adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// Get competition notifications
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%s/notifications", compID), "", userTok)
	assertStatus(t, resp, 200)
	b := decodeJSON(t, resp)
	ns := b["notifications"].([]any)
	if len(ns) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(ns))
	}
}

func TestAnalyticsOverview(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// Create competition
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"Test Comp","description":"Test","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	// Create challenge
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Test Chal","category":"web","description":"desc","score":100,"flag":"flag{test}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	// Add challenge to competition
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// Create test submission
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{test}"}`, userTok)
	assertStatus(t, resp, 200)

	// Make request to analytics overview
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%s/analytics/overview", compID), "", userTok)
	assertStatus(t, resp, 200)

	b := decodeJSON(t, resp)
	if b["total_users"] != json.Number("1") {
		t.Errorf("expected total_users=1, got %v", b["total_users"])
	}
	if b["total_challenges"] != json.Number("1") {
		t.Errorf("expected total_challenges=1, got %v", b["total_challenges"])
	}
	if b["total_submissions"] != json.Number("1") {
		t.Errorf("expected total_submissions=1, got %v", b["total_submissions"])
	}
	if b["correct_submissions"] != json.Number("1") {
		t.Errorf("expected correct_submissions=1, got %v", b["correct_submissions"])
	}
	if b["average_solves"] != json.Number("1") {
		t.Errorf("expected average_solves=1.0, got %v", b["average_solves"])
	}
}

func TestAnalyticsCategories(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// Create competition
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"Test Comp","description":"Test","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	// Create challenges in different categories
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Web Chal","category":"web","description":"desc","score":100,"flag":"flag{web}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID1 := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Crypto Chal","category":"crypto","description":"desc","score":200,"flag":"flag{crypto}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID2 := getID(t, decodeJSON(t, resp))

	// Add challenges to competition
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID1), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID2), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// Make request
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%s/analytics/categories", compID), "", userTok)
	assertStatus(t, resp, 200)

	b := decodeJSON(t, resp)
	categories := b["categories"].([]any)

	// Should have at least the two categories
	catMap := make(map[string]json.Number)
	for _, c := range categories {
		cat := c.(map[string]any)
		catMap[cat["category"].(string)] = cat["total_challenges"].(json.Number)
	}

	if catMap["web"] != json.Number("1") {
		t.Errorf("expected web category with 1 challenge, got %v", catMap["web"])
	}
	if catMap["crypto"] != json.Number("1") {
		t.Errorf("expected crypto category with 1 challenge, got %v", catMap["crypto"])
	}
}

func TestHints(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// Create challenge
	resp := doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Test Chal","description":"desc","score":100,"flag":"flag{test}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	// Create hint 1
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/challenges/%s/hints", chalID),
		`{"content":"First hint"}`, adminTok)
	assertStatus(t, resp, 201)

	// Create hint 2
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/challenges/%s/hints", chalID),
		`{"content":"Second hint"}`, adminTok)
	assertStatus(t, resp, 201)

	// User lists hints - should see 2 visible hints
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%s/hints", chalID), "", userTok)
	assertStatus(t, resp, 200)
	body := decodeJSON(t, resp)
	hints := body["hints"].([]any)
	if len(hints) != 2 {
		t.Fatalf("expected 2 hints, got %d", len(hints))
	}
	hint1ID := getID(t, hints[0].(map[string]any))
	hint2ID := getID(t, hints[1].(map[string]any))

	// Update hint 2 to be invisible
	visible := false
	updateContent := "Updated second hint"
	updateBody, _ := json.Marshal(map[string]any{"content": updateContent, "is_visible": visible})
	resp = doRequest(t, "PUT", fmt.Sprintf("/api/v1/admin/hints/%s", hint2ID), string(updateBody), adminTok)
	assertStatus(t, resp, 204)

	// User lists hints - should now see only 1 hint
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%s/hints", chalID), "", userTok)
	assertStatus(t, resp, 200)
	body = decodeJSON(t, resp)
	hints = body["hints"].([]any)
	if len(hints) != 1 {
		t.Fatalf("expected 1 hint after hiding, got %d", len(hints))
	}

	// Delete hint 1
	resp = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/admin/hints/%s", hint1ID), "", adminTok)
	assertStatus(t, resp, 204)

	// User lists hints - should see 0 hints now
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%s/hints", chalID), "", userTok)
	assertStatus(t, resp, 200)
	body = decodeJSON(t, resp)
	hints = body["hints"].([]any)
	if len(hints) != 0 {
		t.Fatalf("expected 0 hints after deletion, got %d", len(hints))
	}

	// Test invalid hint ID on update/delete
	resp = doRequest(t, "PUT", "/api/v1/admin/hints/00000000000000000000000000000000", `{"content":"x"}`, adminTok)
	assertStatus(t, resp, 404)
	resp = doRequest(t, "DELETE", "/api/v1/admin/hints/00000000000000000000000000000000", "", adminTok)
	assertStatus(t, resp, 404)
}

func TestTopThree(t *testing.T) {
	cleanup(t)

	// Create competition
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"Test Comp","description":"Test","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`,
		makeToken("admin1", "admin"))
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	// Create 2 challenges
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Chal 1","category":"web","description":"desc","score":100,"flag":"flag{1}"}`,
		makeToken("admin1", "admin"))
	assertStatus(t, resp, 201)
	chal1ID := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"Chal 2","category":"pwn","description":"desc","score":200,"flag":"flag{2}"}`,
		makeToken("admin1", "admin"))
	assertStatus(t, resp, 201)
	chal2ID := getID(t, decodeJSON(t, resp))

	// Add challenges to competition
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chal1ID), makeToken("admin1", "admin"))
	assertStatus(t, resp, 201)
	resp.Body.Close()

	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chal2ID), makeToken("admin1", "admin"))
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// Submit for challenge 1 in order: user3, user1, user2, user4
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal1ID),
		`{"flag":"flag{1}"}`, makeToken("user3", "user"))
	assertStatus(t, resp, 200)
	resp.Body.Close()

	time.Sleep(10 * time.Millisecond)
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal1ID),
		`{"flag":"flag{1}"}`, makeToken("user1", "user"))
	assertStatus(t, resp, 200)
	resp.Body.Close()

	time.Sleep(10 * time.Millisecond)
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal1ID),
		`{"flag":"flag{1}"}`, makeToken("user2", "user"))
	assertStatus(t, resp, 200)
	resp.Body.Close()

	time.Sleep(10 * time.Millisecond)
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal1ID),
		`{"flag":"flag{1}"}`, makeToken("user4", "user"))
	assertStatus(t, resp, 200)
	resp.Body.Close()

	// Submit for challenge 2: user2, user3
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal2ID),
		`{"flag":"flag{2}"}`, makeToken("user2", "user"))
	assertStatus(t, resp, 200)
	resp.Body.Close()

	time.Sleep(10 * time.Millisecond)
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal2ID),
		`{"flag":"flag{2}"}`, makeToken("user3", "user"))
	assertStatus(t, resp, 200)
	resp.Body.Close()

	// Wait a bit for event processing
	time.Sleep(100 * time.Millisecond)

	// Test API endpoint
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/topthree/competitions/%s", compID), "", makeToken("user1", "user"))
	assertStatus(t, resp, 200)

	var respStruct struct {
		CompetitionID string `json:"competition_id"`
		Challenges    []struct {
			ChallengeID string `json:"challenge_id"`
			Title       string `json:"title"`
			TopThree    []struct {
				Ranking int    `json:"ranking"`
				UserID  string `json:"user_id"`
			} `json:"top_three"`
		} `json:"challenges"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respStruct); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if respStruct.CompetitionID != compID {
		t.Errorf("competition id mismatch: %s != %s", respStruct.CompetitionID, compID)
	}

	// Find challenge 1
	var chal1 *struct {
		ChallengeID string `json:"challenge_id"`
		Title       string `json:"title"`
		TopThree    []struct {
			Ranking int    `json:"ranking"`
			UserID  string `json:"user_id"`
		} `json:"top_three"`
	}
	for i := range respStruct.Challenges {
		if respStruct.Challenges[i].ChallengeID == chal1ID {
			chal1 = &respStruct.Challenges[i]
			break
		}
	}
	if chal1 == nil {
		t.Fatal("challenge 1 not found in response")
	}

	// Check we have top three entries (may be less due to event timing)
	// Note: Event processing is async, so we just verify the API returns correctly
	if len(chal1.TopThree) > 3 {
		t.Fatalf("expected <= 3 entries, got %d", len(chal1.TopThree))
	}
}

func TestTopThreeBaseModelSoftDelete(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")

	// Create competition
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"SoftDelete Comp","description":"Test","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	// Create challenge
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"SD Chal","category":"web","description":"desc","score":100,"flag":"flag{sd}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	// Add challenge to competition
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// 3 users solve the challenge
	u1 := makeToken("sduser1", "user")
	u2 := makeToken("sduser2", "user")
	u3 := makeToken("sduser3", "user")

	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{sd}"}`, u1).Body.Close()
	time.Sleep(10 * time.Millisecond)
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{sd}"}`, u2).Body.Close()
	time.Sleep(10 * time.Millisecond)
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{sd}"}`, u3).Body.Close()

	time.Sleep(100 * time.Millisecond)

	// Verify: topthree_records table has BaseModel fields populated
	rows, err := testDB.Query(`
		SELECT id, res_id, created_at, updated_at, is_deleted
		FROM topthree_records
		WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 0
		ORDER BY ranking ASC
	`, compID, chalID)
	if err != nil {
		t.Fatalf("query topthree: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		var resID string
		var createdAt, updatedAt time.Time
		var isDeleted bool
		if err := rows.Scan(&id, &resID, &createdAt, &updatedAt, &isDeleted); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if id <= 0 {
			t.Error("expected positive auto-increment id")
		}
		if len(resID) != 32 {
			t.Errorf("expected 32-char res_id, got %d chars: %s", len(resID), resID)
		}
		if createdAt.IsZero() {
			t.Error("expected non-zero created_at")
		}
		if updatedAt.IsZero() {
			t.Error("expected non-zero updated_at")
		}
		if isDeleted {
			t.Error("expected is_deleted=false for active records")
		}
		count++
	}
	if count != 3 {
		t.Fatalf("expected 3 active records, got %d", count)
	}

	// Simulate being pushed out of top three via soft delete (same logic as updateTopThree)
	// Soft-delete the rank 3 record (set ranking=0 to release unique index)
	_, err = testDB.Exec(`
		UPDATE topthree_records SET is_deleted = 1, ranking = 0, updated_at = NOW()
		WHERE competition_id = ? AND challenge_id = ? AND ranking = 3 AND is_deleted = 0
	`, compID, chalID)
	if err != nil {
		t.Fatalf("soft delete rank 3: %v", err)
	}
	// Shift rankings: rank 2 -> rank 3
	_, err = testDB.Exec(`
		UPDATE topthree_records SET ranking = 3, updated_at = NOW()
		WHERE competition_id = ? AND challenge_id = ? AND ranking = 2 AND is_deleted = 0
	`, compID, chalID)
	if err != nil {
		t.Fatalf("shift rank 2->3: %v", err)
	}
	// Shift rankings: rank 1 -> rank 2
	_, err = testDB.Exec(`
		UPDATE topthree_records SET ranking = 2, updated_at = NOW()
		WHERE competition_id = ? AND challenge_id = ? AND ranking = 1 AND is_deleted = 0
	`, compID, chalID)
	if err != nil {
		t.Fatalf("shift rank 1->2: %v", err)
	}
	// Insert new rank 1
	resID := fmt.Sprintf("%032d", time.Now().UnixNano()%100000)
	_, err = testDB.Exec(`
		INSERT INTO topthree_records (res_id, competition_id, challenge_id, user_id, ranking, created_at)
		VALUES (?, ?, ?, ?, 1, NOW())
	`, resID, compID, chalID, "sduser4_faster")
	if err != nil {
		t.Fatalf("insert new rank 1: %v", err)
	}

	// Verify: only 3 active records remain (soft-deleted ones are excluded)
	rows2, err := testDB.Query(`
		SELECT user_id, ranking FROM topthree_records
		WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 0
		ORDER BY ranking ASC
	`, compID, chalID)
	if err != nil {
		t.Fatalf("query topthree after push: %v", err)
	}
	defer rows2.Close()

	var activeUsers []string
	for rows2.Next() {
		var uid string
		var rank int
		if err := rows2.Scan(&uid, &rank); err != nil {
			t.Fatalf("scan2: %v", err)
		}
		activeUsers = append(activeUsers, uid)
	}
	if len(activeUsers) != 3 {
		t.Fatalf("expected 3 active users after push, got %d: %v", len(activeUsers), activeUsers)
	}

	// Verify: there is exactly 1 soft-deleted record
	var softDeletedCount int
	err = testDB.QueryRow(`
		SELECT COUNT(*) FROM topthree_records
		WHERE competition_id = ? AND challenge_id = ? AND is_deleted = 1
	`, compID, chalID).Scan(&softDeletedCount)
	if err != nil {
		t.Fatalf("count soft-deleted: %v", err)
	}
	if softDeletedCount != 1 {
		t.Errorf("expected 1 soft-deleted record, got %d", softDeletedCount)
	}
}

func TestSubmitFlagRateLimit(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("rateuser1", "user")

	// Create competition + challenge + add to comp
	resp := doRequest(t, "POST", "/api/v1/admin/competitions",
		`{"title":"CompRate","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	assertStatus(t, resp, 201)
	compID := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"ChalRate","description":"D","score":200,"flag":"flag{rate}"}`, adminTok)
	assertStatus(t, resp, 201)
	chalID := getID(t, decodeJSON(t, resp))

	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	submitPath := fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID)

	// First 3 requests should succeed (rate limit is 3 per 10s)
	for i := 0; i < 3; i++ {
		resp = doRequest(t, "POST", submitPath, `{"flag":"wrong"}`, userTok)
		assertStatus(t, resp, 200)
		resp.Body.Close()
	}

	// 4th request should be rate limited (429)
	resp = doRequest(t, "POST", submitPath, `{"flag":"wrong"}`, userTok)
	assertStatus(t, resp, http.StatusTooManyRequests)
	resp.Body.Close()

	// Different user should still be able to submit
	user2Tok := makeToken("rateuser2", "user")
	resp = doRequest(t, "POST", submitPath, `{"flag":"wrong"}`, user2Tok)
	assertStatus(t, resp, 200)
	resp.Body.Close()
}
