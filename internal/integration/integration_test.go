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
	"ad7/internal/service"
	"ad7/internal/store"
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
	challengeH := handler.NewChallengeHandler(challengeSvc)
	submissionH := handler.NewSubmissionHandler(submissionSvc)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Authenticate)
		r.Get("/challenges", challengeH.List)
		r.Get("/challenges/{id}", challengeH.Get)
		r.Post("/challenges/{id}/submit", submissionH.Submit)
		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Post("/challenges", challengeH.Create)
			r.Put("/challenges/{id}", challengeH.Update)
			r.Delete("/challenges/{id}", challengeH.Delete)
			r.Get("/submissions", submissionH.List)
		})
	})

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
	testDB.Exec("DELETE FROM submissions")
	testDB.Exec("DELETE FROM challenges")
}

func decodeJSON(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	defer r.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return m
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
	id := int(b["id"].(float64))

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
	id := int(b["id"].(float64))
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
	id := int(b["id"].(float64))
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
	id := int(b["id"].(float64))
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
	id := int(b["id"].(float64))

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
