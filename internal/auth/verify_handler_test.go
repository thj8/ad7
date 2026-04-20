package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVerify_ValidToken(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewVerifyHandler(svc)

	token, err := svc.GenerateToken("user123", "member")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/verify", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.Verify(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["user_id"] != "user123" {
		t.Errorf("user_id = %q, want %q", resp["user_id"], "user123")
	}
	if resp["role"] != "member" {
		t.Errorf("role = %q, want %q", resp["role"], "member")
	}
}

func TestVerify_MissingToken(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewVerifyHandler(svc)

	req := httptest.NewRequest("POST", "/api/v1/verify", nil)
	w := httptest.NewRecorder()

	handler.Verify(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestVerify_InvalidToken(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)
	handler := NewVerifyHandler(svc)

	req := httptest.NewRequest("POST", "/api/v1/verify", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	handler.Verify(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	store := newMockUserStore()
	svc := &AuthService{
		users:     store,
		secret:    []byte("test-secret"),
		adminRole: "admin",
		tokenTTL:  -1 * time.Hour,
	}
	handler := NewVerifyHandler(svc)

	token, _ := svc.GenerateToken("user123", "member")

	req := httptest.NewRequest("POST", "/api/v1/verify", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.Verify(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
