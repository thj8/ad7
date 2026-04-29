package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ErrNotFound 表示资源未找到。
var ErrNotFound = errors.New("not found")

// ErrConflict 表示资源冲突（如用户名已存在）。
var ErrConflict = errors.New("conflict")

// ErrUnauthorized 表示认证失败（用户名或密码错误）。
var ErrUnauthorized = errors.New("invalid username or password")

// AuthService 处理用户注册和登录业务逻辑。
type AuthService struct {
	users      UserStore
	secret     []byte
	adminRole  string
	tokenTTL   time.Duration
}

// NewAuthService 创建 AuthService 实例。
func NewAuthService(users UserStore, secret, adminRole string) *AuthService {
	return &AuthService{
		users:     users,
		secret:    []byte(secret),
		adminRole: adminRole,
		tokenTTL:  24 * time.Hour,
	}
}

// RegisterRequest 是用户注册的请求参数。
type RegisterRequest struct {
	Username string
	Password string
	Role     string // 可选，默认 "member"
}

// Register 注册新用户。
// 使用 bcrypt 加密密码，默认角色为 "member"。
func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*User, error) {
	if req.Username == "" || req.Password == "" {
		return nil, errors.New("username and password are required")
	}
	if len(req.Username) > 255 || len(req.Password) > 255 {
		return nil, errors.New("username or password too long (max 255)")
	}

	// 检查用户名是否已存在
	existing, err := s.users.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("check username: %w", err)
	}
	if existing != nil {
		return nil, ErrConflict
	}

	// 加密密码
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	role := "member" // 注册强制为 member，忽略用户传入的 role

	user := &User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         role,
	}

	id, err := s.users.CreateUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	user.ResID = id
	return user, nil
}

// LoginRequest 是用户登录的请求参数。
type LoginRequest struct {
	Username string
	Password string
}

// Login 验证用户名和密码，返回 JWT token。
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (string, error) {
	if req.Username == "" || req.Password == "" {
		return "", errors.New("username and password are required")
	}

	user, err := s.users.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return "", ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return "", ErrUnauthorized
	}

	return s.GenerateToken(user.ResID, user.Username, user.Role)
}

// GenerateToken 生成 JWT token。
func (s *AuthService) GenerateToken(userID, username, role string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      userID,
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(s.tokenTTL).Unix(),
	})
	return token.SignedString(s.secret)
}

// VerifyToken 验证 JWT token，返回 user_id、username 和 role。
func (s *AuthService) VerifyToken(tokenStr string) (string, string, string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return "", "", "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", "", jwt.ErrSignatureInvalid
	}
	userID, _ := claims["sub"].(string)
	username, _ := claims["username"].(string)
	role, _ := claims["role"].(string)
	if userID == "" || role == "" {
		return "", "", "", jwt.ErrSignatureInvalid
	}
	return userID, username, role, nil
}
