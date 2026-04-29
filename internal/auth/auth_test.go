package auth

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// mockUserStore 是 UserStore 的内存模拟实现。
type mockUserStore struct {
	users map[string]*User // by username
	byID  map[string]*User // by res_id
	nextID int
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{
		users: make(map[string]*User),
		byID:  make(map[string]*User),
	}
}

func (m *mockUserStore) CreateUser(_ context.Context, u *User) (string, error) {
	u.ResID = "mock_id_" + string(rune('a'+m.nextID))
	m.nextID++
	u2 := *u
	m.users[u.Username] = &u2
	m.byID[u2.ResID] = &u2
	return u2.ResID, nil
}

func (m *mockUserStore) GetUserByUsername(_ context.Context, username string) (*User, error) {
	u, ok := m.users[username]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserStore) GetUserByID(_ context.Context, resID string) (*User, error) {
	u, ok := m.byID[resID]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserStore) ListUsersByTeam(_ context.Context, teamID string) ([]User, error) {
	return []User{}, nil
}

func (m *mockUserStore) SetTeamID(_ context.Context, userID, teamID string) error {
	return nil
}

func (m *mockUserStore) ListUsersByResIDs(_ context.Context, resIDs []string) ([]User, error) {
	var result []User
	for _, id := range resIDs {
		if u, ok := m.byID[id]; ok {
			result = append(result, *u)
		}
	}
	return result, nil
}

func (m *mockUserStore) DeleteUser(_ context.Context, resID string) error {
	delete(m.byID, resID)
	return nil
}

func newTestAuthService(store UserStore) *AuthService {
	return &AuthService{
		users:    store,
		secret:   []byte("test-secret"),
		adminRole: "admin",
		tokenTTL: 3600e9,
	}
}

func TestRegister_Success(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)

	user, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "player1",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.Username != "player1" {
		t.Errorf("username = %q, want %q", user.Username, "player1")
	}
	if user.Role != "member" {
		t.Errorf("role = %q, want %q", user.Role, "member")
	}
	if user.ResID == "" {
		t.Error("res_id should not be empty")
	}
	// 验证密码哈希
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("secret123")); err != nil {
		t.Errorf("password hash mismatch: %v", err)
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)

	_, _ = svc.Register(context.Background(), &RegisterRequest{
		Username: "player1",
		Password: "secret123",
	})

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "player1",
		Password: "another",
	})
	if err != ErrConflict {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestRegister_EmptyFields(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)

	_, err := svc.Register(context.Background(), &RegisterRequest{Username: "", Password: "pass"})
	if err == nil {
		t.Error("expected error for empty username")
	}

	_, err = svc.Register(context.Background(), &RegisterRequest{Username: "user", Password: ""})
	if err == nil {
		t.Error("expected error for empty password")
	}
}

func TestLogin_Success(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)

	_, _ = svc.Register(context.Background(), &RegisterRequest{
		Username: "player1",
		Password: "secret123",
	})

	token, err := svc.Login(context.Background(), &LoginRequest{
		Username: "player1",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)

	_, _ = svc.Register(context.Background(), &RegisterRequest{
		Username: "player1",
		Password: "secret123",
	})

	_, err := svc.Login(context.Background(), &LoginRequest{
		Username: "player1",
		Password: "wrong",
	})
	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)

	_, err := svc.Login(context.Background(), &LoginRequest{
		Username: "nonexistent",
		Password: "secret123",
	})
	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestRegister_AdminRoleEscalation(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)

	// Attack: try to register with role=admin
	user, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "hacker",
		Password: "secret123",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.Role == "admin" {
		t.Error("SECURITY: user should NOT be able to self-assign admin role via registration")
	}
	if user.Role != "member" {
		t.Errorf("role = %q, want %q", user.Role, "member")
	}
}

func TestGenerateToken(t *testing.T) {
	store := newMockUserStore()
	svc := newTestAuthService(store)

	token, err := svc.GenerateToken("user123", "testuser", "admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
}
