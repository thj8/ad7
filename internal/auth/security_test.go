package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"ad7/internal/middleware"
)

// newTestServer creates a test server with full auth middleware stack.
func newTestServer(t *testing.T) (*httptest.Server, *mockUserStore) {
	t.Helper()
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewAuthHandler(svc)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.Throttle(100))
	r.Use(middleware.MaxBodySize(1 << 20)) // 1MB

	// Rate limited routes (5 requests per second for testing)
	r.Group(func(r chi.Router) {
		r.Use(middleware.LimitAuthEndpoints(5, time.Second))
		r.Post("/register", handler.Register)
		r.Post("/login", handler.Login)
	})

	return httptest.NewServer(r), store
}

// --- Rate limiting tests (H-01) ---

func TestRateLimit_Login(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	// Register a user first
	body, _ := json.Marshal(map[string]string{"username": "ratelimituser", "password": "pass123"})
	resp, _ := srv.Client().Post(srv.URL+"/register", "application/json", bytes.NewReader(body))
	resp.Body.Close()

	limited := false
	for range 20 {
		resp, _ := srv.Client().Post(srv.URL+"/login", "application/json",
			bytes.NewReader([]byte(`{"username":"ratelimituser","password":"wrong"}`)))
		if resp.StatusCode == http.StatusTooManyRequests {
			limited = true
			break
		}
		resp.Body.Close()
	}
	if !limited {
		t.Errorf("SECURITY: login should be rate limited after 20 attempts (only got %d through)", 20)
	}
}

func TestRateLimit_Registration(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	limited := false
	for i := range 20 {
		body := []byte(`{"username":"spam_` + strings.Repeat("a", i%10) + `","password":"pass123"}`)
		resp, _ := srv.Client().Post(srv.URL+"/register", "application/json", bytes.NewReader(body))
		if resp.StatusCode == http.StatusTooManyRequests {
			limited = true
			break
		}
		resp.Body.Close()
	}
	if !limited {
		t.Error("SECURITY: registration should be rate limited after many attempts")
	}
}

// --- Body size limit tests (M-01) ---

func TestMaxBodySize_Rejected(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewAuthHandler(svc)

	r := chi.NewRouter()
	r.Use(middleware.MaxBodySize(100)) // Very small limit for testing
	r.Post("/register", handler.Register)

	bigBody := `{"username":"` + strings.Repeat("x", 200) + `","password":"pass"}`
	req := httptest.NewRequest("POST", "/register", strings.NewReader(bigBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusCreated {
		t.Error("SECURITY: oversized request should not be created successfully")
	}
}

func TestMaxBodySize_NormalRequest(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewAuthHandler(svc)

	r := chi.NewRouter()
	r.Use(middleware.MaxBodySize(1024))
	r.Post("/register", handler.Register)

	body, _ := json.Marshal(map[string]string{"username": "normal", "password": "pass123"})
	req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusRequestEntityTooLarge {
		t.Errorf("normal request should not be rejected as too large")
	}
}

// --- User enumeration prevention (M-03) ---

func TestRegister_NoUsernameEnumeration(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewAuthHandler(svc)

	// Register first user
	regBody := `{"username":"existinguser","password":"pass123"}`
	req1 := httptest.NewRequest("POST", "/register", strings.NewReader(regBody))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.Register(w1, req1)

	// Try to register same username again
	req2 := httptest.NewRequest("POST", "/register", strings.NewReader(regBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.Register(w2, req2)

	if w2.Code == http.StatusConflict {
		t.Error("SECURITY: duplicate username should not return 409 Conflict (enables enumeration)")
	}
	// Should return 400 with generic message, not revealing the specific reason
	if w2.Code != http.StatusBadRequest {
		t.Errorf("duplicate username should return 400, got %d", w2.Code)
	}

	var resp map[string]string
	json.NewDecoder(w2.Body).Decode(&resp)
	if resp["error"] == "username already exists" {
		t.Error("SECURITY: error message reveals username exists")
	}
}

// --- Internal error sanitization (M-04) ---

type errorStore struct {
	mockUserStore
}

func (e *errorStore) GetUserByUsername(_ context.Context, _ string) (*User, error) {
	return nil, fmt.Errorf("database connection lost: SQLSTATE 08006")
}

func TestLogin_InternalErrorSanitized(t *testing.T) {
	store := &errorStore{mockUserStore: *newMockUserStore()}
	svc := newTestAuthService(store)
	handler := NewAuthHandler(svc)

	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"test","password":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Login(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for internal error, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	errMsg := resp["error"]
	if strings.Contains(errMsg, "SQLSTATE") || strings.Contains(errMsg, "database") {
		t.Errorf("SECURITY: internal error leaked to client: %s", errMsg)
	}
}

func TestRegister_InternalErrorSanitized(t *testing.T) {
	store := &errorStore{mockUserStore: *newMockUserStore()}
	svc := newTestAuthService(store)
	handler := NewAuthHandler(svc)

	req := httptest.NewRequest("POST", "/register", strings.NewReader(`{"username":"test","password":"test123"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Register(w, req)

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	errMsg := resp["error"]
	if strings.Contains(errMsg, "SQLSTATE") || strings.Contains(errMsg, "database") {
		t.Errorf("SECURITY: internal error leaked to client: %s", errMsg)
	}
}
