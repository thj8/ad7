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
	"ad7/plugins/leaderboard"
	"ad7/plugins/notification"
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
		r.Post("/challenges/{id}/submit", submissionH.Submit)
		r.Get("/competitions", compH.List)
		r.Get("/competitions/{id}", compH.Get)
		r.Get("/competitions/{id}/challenges", compH.ListChallenges)
		r.Post("/competitions/{comp_id}/challenges/{id}/submit", submissionH.SubmitInComp)
		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Post("/challenges", challengeH.Create)
			r.Put("/challenges/{id}", challengeH.Update)
			r.Delete("/challenges/{id}", challengeH.Delete)
			r.Get("/submissions", submissionH.List)
			r.Post("/competitions", compH.Create)
			r.Get("/competitions", compH.ListAll)
			r.Put("/competitions/{id}", compH.Update)
			r.Delete("/competitions/{id}", compH.Delete)
			r.Post("/competitions/{id}/challenges", compH.AddChallenge)
			r.Delete("/competitions/{id}/challenges/{challenge_id}", compH.RemoveChallenge)
		})
	})

	plugins := []plugin.Plugin{leaderboard.New(), notification.New()}
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

func getID(t *testing.T, m map[string]any) int64 {
	t.Helper()
	n, ok := m["id"].(json.Number)
	if !ok {
		t.Fatalf("id not a json.Number: %T %v", m["id"], m["id"])
	}
	v, err := n.Int64()
	if err != nil {
		t.Fatalf("id parse: %v", err)
	}
	return v
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
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%d", id), "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["title"] != "T1" {
		t.Fatalf("expected title T1, got %v", b["title"])
	}
	if _, hasFlag := b["flag"]; hasFlag {
		t.Fatal("flag must not be in response")
	}

	// 404 not found
	resp = doRequest(t, "GET", "/api/v1/challenges/99999", "", userTok)
	assertStatus(t, resp, 404)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/challenges/%d", id), "", "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestSubmitFlag(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// create challenge
	resp := doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"T2","description":"D","score":100,"flag":"flag{correct}"}`, adminTok)
	assertStatus(t, resp, 201)
	b := decodeJSON(t, resp)
	id := getID(t, b)
	path := fmt.Sprintf("/api/v1/challenges/%d/submit", id)

	// wrong flag
	resp = doRequest(t, "POST", path, `{"flag":"flag{wrong}"}`, userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["success"] != false {
		t.Fatal("expected success=false for wrong flag")
	}
	if b["message"] != "incorrect" {
		t.Fatalf("expected message=incorrect, got %v", b["message"])
	}

	// correct flag
	resp = doRequest(t, "POST", path, `{"flag":"flag{correct}"}`, userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["success"] != true {
		t.Fatal("expected success=true for correct flag")
	}

	// already solved
	resp = doRequest(t, "POST", path, `{"flag":"flag{correct}"}`, userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["message"] != "already_solved" {
		t.Fatalf("expected already_solved, got %v", b["message"])
	}

	// 400 missing flag
	resp = doRequest(t, "POST", path, `{}`, userTok)
	assertStatus(t, resp, 400)
	resp.Body.Close()

	// 404 bad id
	resp = doRequest(t, "POST", "/api/v1/challenges/99999/submit", `{"flag":"x"}`, userTok)
	assertStatus(t, resp, 404)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "POST", path, `{"flag":"flag{correct}"}`, "")
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
	path := fmt.Sprintf("/api/v1/admin/challenges/%d", id)

	// 204 update
	resp = doRequest(t, "PUT", path,
		`{"title":"T4-updated","description":"D","score":150,"flag":"flag{u}","is_enabled":true}`, adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// 404
	resp = doRequest(t, "PUT", "/api/v1/admin/challenges/99999",
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
	path := fmt.Sprintf("/api/v1/admin/challenges/%d", id)

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
}

func TestAdminListSubmissions(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// create challenge and submit
	resp := doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"T6","description":"D","score":100,"flag":"flag{s}"}`, adminTok)
	assertStatus(t, resp, 201)
	b := decodeJSON(t, resp)
	id := getID(t, b)

	doRequest(t, "POST", fmt.Sprintf("/api/v1/challenges/%d/submit", id),
		`{"flag":"flag{s}"}`, userTok)

	// 200 unfiltered
	resp = doRequest(t, "GET", "/api/v1/admin/submissions", "", adminTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	subs := b["submissions"].([]any)
	if len(subs) == 0 {
		t.Fatal("expected at least 1 submission")
	}

	// 200 filtered by user_id
	resp = doRequest(t, "GET", "/api/v1/admin/submissions?user_id=user1", "", adminTok)
	assertStatus(t, resp, 200)
	resp.Body.Close()

	// 200 filtered by challenge_id
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/admin/submissions?challenge_id=%d", id), "", adminTok)
	assertStatus(t, resp, 200)
	resp.Body.Close()

	// 403
	resp = doRequest(t, "GET", "/api/v1/admin/submissions", "", userTok)
	assertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "GET", "/api/v1/admin/submissions", "", "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestLeaderboard(t *testing.T) {
	cleanup(t)
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	resp := doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"LB","description":"D","score":100,"flag":"flag{lb}"}`, adminTok)
	assertStatus(t, resp, 201)
	b := decodeJSON(t, resp)
	id := getID(t, b)

	doRequest(t, "PUT", fmt.Sprintf("/api/v1/admin/challenges/%d", id),
		`{"title":"LB","description":"D","score":100,"flag":"flag{lb}","is_enabled":true}`, adminTok).Body.Close()
	doRequest(t, "POST", fmt.Sprintf("/api/v1/challenges/%d/submit", id),
		`{"flag":"flag{lb}"}`, userTok).Body.Close()

	resp = doRequest(t, "GET", "/api/v1/leaderboard", "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	board := b["leaderboard"].([]any)
	if len(board) == 0 {
		t.Fatal("expected at least 1 leaderboard entry")
	}
	if board[0].(map[string]any)["user_id"] != "user1" {
		t.Fatalf("expected user1 at rank 1")
	}

	resp = doRequest(t, "GET", "/api/v1/leaderboard", "", "")
	assertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestNotifications(t *testing.T) {
	cleanup(t)
	testDB.Exec("DELETE FROM notifications")
	adminTok := makeToken("admin1", "admin")
	userTok := makeToken("user1", "user")

	// global notification
	resp := doRequest(t, "POST", "/api/v1/admin/notifications",
		`{"title":"比赛开始","message":"祝好运"}`, adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	resp = doRequest(t, "GET", "/api/v1/notifications", "", userTok)
	assertStatus(t, resp, 200)
	b := decodeJSON(t, resp)
	if len(b["notifications"].([]any)) == 0 {
		t.Fatal("expected global notification")
	}

	// per-challenge notification
	resp = doRequest(t, "POST", "/api/v1/admin/challenges",
		`{"title":"N1","description":"D","score":100,"flag":"flag{n}"}`, adminTok)
	assertStatus(t, resp, 201)
	cb := decodeJSON(t, resp)
	cid := getID(t, cb)

	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/challenges/%d/notifications", cid),
		`{"title":"提示","message":"看看源码"}`, adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// not visible before solving
	resp = doRequest(t, "GET", "/api/v1/notifications", "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if len(b["notifications"].([]any)) != 1 {
		t.Fatal("should only see global notification before solving")
	}

	// solve then visible
	doRequest(t, "PUT", fmt.Sprintf("/api/v1/admin/challenges/%d", cid),
		`{"title":"N1","description":"D","score":100,"flag":"flag{n}","is_enabled":true}`, adminTok).Body.Close()
	doRequest(t, "POST", fmt.Sprintf("/api/v1/challenges/%d/submit", cid),
		`{"flag":"flag{n}"}`, userTok).Body.Close()

	resp = doRequest(t, "GET", "/api/v1/notifications", "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if len(b["notifications"].([]any)) != 2 {
		t.Fatalf("expected 2 notifications after solve, got %d", len(b["notifications"].([]any)))
	}

	// 403
	resp = doRequest(t, "POST", "/api/v1/admin/notifications",
		`{"title":"x","message":"y"}`, userTok)
	assertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = doRequest(t, "GET", "/api/v1/notifications", "", "")
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
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%d", compID), "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	if b["title"] != "CTF Round 1" {
		t.Fatalf("expected title 'CTF Round 1', got %v", b["title"])
	}

	// Update competition
	resp = doRequest(t, "PUT", fmt.Sprintf("/api/v1/admin/competitions/%d", compID),
		`{"title":"CTF Round 1 Updated","description":"Updated","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z","is_active":true}`, adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// Delete competition
	resp = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/admin/competitions/%d", compID), "", adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// 404 after delete
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%d", compID), "", userTok)
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
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%d/challenges", compID),
		fmt.Sprintf(`{"challenge_id":%d}`, chalID), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// List competition challenges
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%d/challenges", compID), "", userTok)
	assertStatus(t, resp, 200)
	b := decodeJSON(t, resp)
	chals := b["challenges"].([]any)
	if len(chals) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(chals))
	}

	// Remove challenge from competition
	resp = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/admin/competitions/%d/challenges/%d", compID, chalID), "", adminTok)
	assertStatus(t, resp, 204)
	resp.Body.Close()

	// Verify empty
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%d/challenges", compID), "", userTok)
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

	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%d/challenges", compID),
		fmt.Sprintf(`{"challenge_id":%d}`, chalID), adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	submitPath := fmt.Sprintf("/api/v1/competitions/%d/challenges/%d/submit", compID, chalID)

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
	doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%d/challenges", compID),
		fmt.Sprintf(`{"challenge_id":%d}`, ch1), adminTok).Body.Close()
	doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%d/challenges", compID),
		fmt.Sprintf(`{"challenge_id":%d}`, ch2), adminTok).Body.Close()

	// user1 solves both
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%d/challenges/%d/submit", compID, ch1),
		`{"flag":"flag{1}"}`, u1).Body.Close()
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%d/challenges/%d/submit", compID, ch2),
		`{"flag":"flag{2}"}`, u1).Body.Close()

	// user2 solves only ch1
	doRequest(t, "POST", fmt.Sprintf("/api/v1/competitions/%d/challenges/%d/submit", compID, ch1),
		`{"flag":"flag{1}"}`, u2).Body.Close()

	// Competition leaderboard: user1=300, user2=100
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%d/leaderboard", compID), "", u1)
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

	// Global leaderboard should be empty (no non-competition submissions)
	resp = doRequest(t, "GET", "/api/v1/leaderboard", "", u1)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	globalBoard := b["leaderboard"].([]any)
	if len(globalBoard) != 0 {
		t.Fatalf("global leaderboard should be empty, got %d entries", len(globalBoard))
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
	resp = doRequest(t, "POST", fmt.Sprintf("/api/v1/admin/competitions/%d/notifications", compID),
		`{"title":"比赛提示","message":"注意时间"}`, adminTok)
	assertStatus(t, resp, 201)
	resp.Body.Close()

	// Get competition notifications
	resp = doRequest(t, "GET", fmt.Sprintf("/api/v1/competitions/%d/notifications", compID), "", userTok)
	assertStatus(t, resp, 200)
	b := decodeJSON(t, resp)
	ns := b["notifications"].([]any)
	if len(ns) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(ns))
	}

	// Global notifications should NOT include competition notifications
	resp = doRequest(t, "GET", "/api/v1/notifications", "", userTok)
	assertStatus(t, resp, 200)
	b = decodeJSON(t, resp)
	globalNs := b["notifications"].([]any)
	if len(globalNs) != 0 {
		t.Fatalf("global notifications should be empty, got %d", len(globalNs))
	}
}
