// Package testutil provides shared integration test infrastructure for the ad7 project.
// It encapsulates database setup, HTTP test server creation, JWT token generation,
// and common request/assertion helpers used across integration test packages.
package testutil

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"

	"ad7/internal/auth"
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

// TestSecret is the JWT signing key used in integration tests.
const TestSecret = "test-secret"

// AdminRole is the role name that grants admin access.
const AdminRole = "admin"

// DSN returns the MySQL data source name from the TEST_DSN environment variable.
// If TEST_DSN is not set, it returns a default local development DSN.
func DSN() string {
	if v := os.Getenv("TEST_DSN"); v != "" {
		return v
	}
	log.Fatal("TEST_DSN environment variable is required")
	return ""
}

// TestEnv holds the shared test infrastructure: an HTTP test server and a database
// connection. It is created by NewTestEnv and must be closed with Close when tests
// are done.
type TestEnv struct {
	Server     *httptest.Server
	AuthServer *httptest.Server
	DB         *sql.DB
	store      *store.Store
}

// NewTestEnv creates a fully wired test environment suitable for TestMain.
// It connects to the database using DSN(), assembles the complete router (identical
// to the production route layout), starts an httptest.Server, and returns a TestEnv
// ready for use.
//
// The caller is responsible for calling Close() on the returned TestEnv, typically
// via defer in TestMain.
func NewTestEnv(m *testing.M) *TestEnv {
	st, err := store.New(DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect db: %v\n", err)
		os.Exit(1)
	}

	// Rate limit configuration: 3 requests per 10 seconds on submit endpoints.
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

	// 启动 auth 测试服务器
	authStore := auth.NewAuthStore(st.DB())
	authSvc := auth.NewAuthService(authStore, TestSecret, AdminRole)
	verifyH := auth.NewVerifyHandler(authSvc)
	authDeps := auth.RouteDeps{AuthH: auth.NewAuthHandler(authSvc), TeamH: auth.NewTeamHandler(auth.NewTeamService(authStore, authStore))}
	authR := chi.NewRouter()
	authR.Use(chimw.Recoverer)
	authR.Route("/api/v1", func(r chi.Router) {
		auth.RegisterPublicRoutes(r, authDeps)
		r.Post("/verify", verifyH.Verify)
	})
	authServer := httptest.NewServer(authR)

	auth := middleware.NewAuth(authServer.URL, AdminRole)
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
			r.Post("/competitions/{id}/start", compH.Start)
			r.Post("/competitions/{id}/end", compH.End)
		})
	})

	plugins := []plugin.Plugin{leaderboard.New(), notification.New(), analytics.New(), hints.New(), topthree.New()}

	// 构建插件名称到实例的映射，用于依赖注入
	pluginMap := make(map[string]plugin.Plugin)
	for _, p := range plugins {
		pluginMap[p.Name()] = p
	}

	for _, p := range plugins {
		p.Register(r, st.DB(), auth, pluginMap)
	}

	return &TestEnv{
		Server:     httptest.NewServer(r),
		AuthServer: authServer,
		DB:         st.DB(),
		store:      st,
	}
}

// Close shuts down the HTTP test server and closes the store (database connection).
func (e *TestEnv) Close() {
	e.Server.Close()
	e.AuthServer.Close()
	e.store.Close()
}

// Cleanup deletes all rows from the integration test tables in dependency order.
// Call this at the start of each test to ensure a clean state.
func Cleanup(t *testing.T, db *sql.DB) {
	t.Helper()
	db.Exec("DELETE FROM topthree_records")
	db.Exec("DELETE FROM hints")
	db.Exec("DELETE FROM competition_challenges")
	db.Exec("DELETE FROM notifications")
	db.Exec("DELETE FROM submissions")
	db.Exec("DELETE FROM competitions")
	db.Exec("DELETE FROM challenges")
	db.Exec("DELETE FROM users")
	db.Exec("DELETE FROM teams")
}

// MakeToken creates a JWT token valid for 1 hour with the given userID and role.
func MakeToken(userID, role string) string {
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(TestSecret))
	if err != nil {
		panic(fmt.Sprintf("sign token failed: %v", err))
	}
	return tok
}

// MakeExpiredToken creates a JWT token that has already expired.
func MakeExpiredToken(userID, role string) string {
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(-time.Hour).Unix(),
	}).SignedString([]byte(TestSecret))
	if err != nil {
		panic(fmt.Sprintf("sign expired token failed: %v", err))
	}
	return tok
}

// DoRequest sends an HTTP request to the test server and returns the response.
// serverURL is the base URL of the test server (e.g. env.Server.URL).
// If body is non-empty, it sets Content-Type to application/json.
// If token is non-empty, it sets the Authorization header.
func DoRequest(t *testing.T, serverURL, method, path, body, token string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, serverURL+path, bodyReader)
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

// DecodeJSON decodes the response body into a map[string]any using json.Number
// for numeric values. It closes the response body.
func DecodeJSON(t *testing.T, r *http.Response) map[string]any {
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

// GetID extracts the "id" field from a decoded JSON response map as a string.
func GetID(t *testing.T, m map[string]any) string {
	t.Helper()
	id, ok := m["id"].(string)
	if !ok {
		t.Fatalf("id not a string: %T %v", m["id"], m["id"])
	}
	return id
}

// AssertStatus checks that the response status code matches the expected value.
// On mismatch, it reads and displays the response body in the failure message.
func AssertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("want status %d, got %d: %s", want, resp.StatusCode, body)
	}
}
