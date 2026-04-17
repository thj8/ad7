# CTF 解题赛系统 - 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task.

**Goal:** CTF Jeopardy 后端 API 服务 — 题目管理 + 静态 flag 验证。

**Tech Stack:** Go, go-chi/chi/v5, database/sql + go-sql-driver/mysql, golang-jwt/jwt/v5, gopkg.in/yaml.v3

**Module path:** `ad7`

---

## Task 1: Project Scaffold

- [ ] Create `go.mod`
- [ ] Install dependencies
- [ ] Create `config.yaml`
- [ ] Create `sql/schema.sql`

### go.mod
```
module ad7

go 1.22
```

### Install deps
```bash
cd /Users/sugar/src/project/ad7
go get github.com/go-chi/chi/v5
go get github.com/go-sql-driver/mysql
go get github.com/golang-jwt/jwt/v5
go get gopkg.in/yaml.v3
```

### config.yaml
```yaml
server:
  port: 8080

db:
  host: 127.0.0.1
  port: 3306
  user: root
  password: ""
  dbname: ctf

jwt:
  secret: "change-me-in-production"
  admin_role: "admin"
```

### sql/schema.sql
```sql
CREATE DATABASE IF NOT EXISTS ctf CHARACTER SET utf8mb4;
USE ctf;

CREATE TABLE IF NOT EXISTS challenges (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    category    VARCHAR(64)  NOT NULL DEFAULT 'misc',
    description TEXT         NOT NULL,
    score       INT          NOT NULL DEFAULT 100,
    flag        VARCHAR(255) NOT NULL,
    is_enabled  TINYINT(1)   NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS submissions (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    user_id        VARCHAR(128) NOT NULL,
    challenge_id   INT          NOT NULL,
    submitted_flag VARCHAR(255) NOT NULL,
    is_correct     TINYINT(1)   NOT NULL,
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (challenge_id) REFERENCES challenges(id),
    INDEX idx_user_challenge (user_id, challenge_id)
);
```

---

## Task 2: Config + Models

- [ ] Create `internal/config/config.go`
- [ ] Create `internal/model/model.go`

### internal/config/config.go
```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	DB     DBConfig     `yaml:"db"`
	JWT    JWTConfig    `yaml:"jwt"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

func (d *DBConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		d.User, d.Password, d.Host, d.Port, d.DBName)
}

type JWTConfig struct {
	Secret    string `yaml:"secret"`
	AdminRole string `yaml:"admin_role"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.JWT.AdminRole == "" {
		cfg.JWT.AdminRole = "admin"
	}
	return &cfg, nil
}
```

### internal/model/model.go
```go
package model

import "time"

type Challenge struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Score       int       `json:"score"`
	Flag        string    `json:"-"`
	IsEnabled   bool      `json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Submission struct {
	ID            int       `json:"id"`
	UserID        string    `json:"user_id"`
	ChallengeID   int       `json:"challenge_id"`
	SubmittedFlag string    `json:"submitted_flag"`
	IsCorrect     bool      `json:"is_correct"`
	CreatedAt     time.Time `json:"created_at"`
}
```

---

## Task 3: Store Interfaces + MySQL Implementation

- [ ] Create `internal/store/store.go`
- [ ] Create `internal/store/mysql.go`

### internal/store/store.go
```go
package store

import (
	"context"

	"ad7/internal/model"
)

type ChallengeStore interface {
	ListEnabled(ctx context.Context) ([]model.Challenge, error)
	GetEnabledByID(ctx context.Context, id int) (*model.Challenge, error)
	GetByID(ctx context.Context, id int) (*model.Challenge, error)
	Create(ctx context.Context, c *model.Challenge) (int64, error)
	Update(ctx context.Context, c *model.Challenge) error
	Delete(ctx context.Context, id int) error
}

type SubmissionStore interface {
	HasCorrectSubmission(ctx context.Context, userID string, challengeID int) (bool, error)
	Create(ctx context.Context, s *model.Submission) error
	List(ctx context.Context, userID string, challengeID int) ([]model.Submission, error)
}
```

### internal/store/mysql.go
```go
package store

import (
	"context"
	"database/sql"
	"fmt"

	"ad7/internal/model"
	_ "github.com/go-sql-driver/mysql"
)

type Store struct {
	db *sql.DB
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// ChallengeStore implementation

func (s *Store) ListEnabled(ctx context.Context) ([]model.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, category, description, score, is_enabled, created_at, updated_at
		 FROM challenges WHERE is_enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []model.Challenge
	for rows.Next() {
		var c model.Challenge
		if err := rows.Scan(&c.ID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

func (s *Store) GetEnabledByID(ctx context.Context, id int) (*model.Challenge, error) {
	var c model.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, category, description, score, is_enabled, created_at, updated_at
		 FROM challenges WHERE id = ? AND is_enabled = 1`, id).
		Scan(&c.ID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (s *Store) GetByID(ctx context.Context, id int) (*model.Challenge, error) {
	var c model.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, category, description, score, flag, is_enabled, created_at, updated_at
		 FROM challenges WHERE id = ?`, id).
		Scan(&c.ID, &c.Title, &c.Category, &c.Description,
			&c.Score, &c.Flag, &c.IsEnabled, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (s *Store) Create(ctx context.Context, c *model.Challenge) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO challenges (title, category, description, score, flag, is_enabled)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.Title, c.Category, c.Description, c.Score, c.Flag, c.IsEnabled)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) Update(ctx context.Context, c *model.Challenge) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE challenges SET title=?, category=?, description=?, score=?, flag=?, is_enabled=?
		 WHERE id=?`,
		c.Title, c.Category, c.Description, c.Score, c.Flag, c.IsEnabled, c.ID)
	return err
}

func (s *Store) Delete(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM challenges WHERE id = ?`, id)
	return err
}

// SubmissionStore implementation

func (s *Store) HasCorrectSubmission(ctx context.Context, userID string, challengeID int) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct=1`,
		userID, challengeID).Scan(&count)
	return count > 0, err
}

func (s *Store) Create(ctx context.Context, sub *model.Submission) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO submissions (user_id, challenge_id, submitted_flag, is_correct)
		 VALUES (?, ?, ?, ?)`,
		sub.UserID, sub.ChallengeID, sub.SubmittedFlag, sub.IsCorrect)
	return err
}

func (s *Store) List(ctx context.Context, userID string, challengeID int) ([]model.Submission, error) {
	query := `SELECT id, user_id, challenge_id, submitted_flag, is_correct, created_at FROM submissions WHERE 1=1`
	args := []any{}
	if userID != "" {
		query += " AND user_id=?"
		args = append(args, userID)
	}
	if challengeID > 0 {
		query += " AND challenge_id=?"
		args = append(args, challengeID)
	}
	query += " ORDER BY created_at DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []model.Submission
	for rows.Next() {
		var sub model.Submission
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.ChallengeID,
			&sub.SubmittedFlag, &sub.IsCorrect, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}
```

---

## Task 4: Services

- [ ] Create `internal/service/challenge.go`
- [ ] Create `internal/service/submission.go`

### internal/service/challenge.go
```go
package service

import (
	"context"
	"errors"

	"ad7/internal/model"
	"ad7/internal/store"
)

var ErrNotFound = errors.New("not found")

type ChallengeService struct {
	store store.ChallengeStore
}

func NewChallengeService(s store.ChallengeStore) *ChallengeService {
	return &ChallengeService{store: s}
}

func (s *ChallengeService) List(ctx context.Context) ([]model.Challenge, error) {
	return s.store.ListEnabled(ctx)
}

func (s *ChallengeService) Get(ctx context.Context, id int) (*model.Challenge, error) {
	c, err := s.store.GetEnabledByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	return c, nil
}

func (s *ChallengeService) Create(ctx context.Context, c *model.Challenge) (int64, error) {
	if c.Title == "" || c.Flag == "" {
		return 0, errors.New("title and flag are required")
	}
	if c.Score <= 0 {
		c.Score = 100
	}
	if c.Category == "" {
		c.Category = "misc"
	}
	c.IsEnabled = true
	return s.store.Create(ctx, c)
}

func (s *ChallengeService) Update(ctx context.Context, id int, patch *model.Challenge) error {
	existing, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	if patch.Title != "" {
		existing.Title = patch.Title
	}
	if patch.Category != "" {
		existing.Category = patch.Category
	}
	if patch.Description != "" {
		existing.Description = patch.Description
	}
	if patch.Score > 0 {
		existing.Score = patch.Score
	}
	if patch.Flag != "" {
		existing.Flag = patch.Flag
	}
	existing.IsEnabled = patch.IsEnabled
	return s.store.Update(ctx, existing)
}

func (s *ChallengeService) Delete(ctx context.Context, id int) error {
	return s.store.Delete(ctx, id)
}
```

### internal/service/submission.go
```go
package service

import (
	"context"
	"errors"

	"ad7/internal/model"
	"ad7/internal/store"
)

type SubmitResult string

const (
	ResultCorrect      SubmitResult = "correct"
	ResultIncorrect    SubmitResult = "incorrect"
	ResultAlreadySolved SubmitResult = "already_solved"
)

type SubmissionService struct {
	challenges  store.ChallengeStore
	submissions store.SubmissionStore
}

func NewSubmissionService(c store.ChallengeStore, s store.SubmissionStore) *SubmissionService {
	return &SubmissionService{challenges: c, submissions: s}
}

func (s *SubmissionService) Submit(ctx context.Context, userID string, challengeID int, flag string) (SubmitResult, error) {
	solved, err := s.submissions.HasCorrectSubmission(ctx, userID, challengeID)
	if err != nil {
		return "", err
	}
	if solved {
		return ResultAlreadySolved, nil
	}

	challenge, err := s.challenges.GetByID(ctx, challengeID)
	if err != nil {
		return "", err
	}
	if challenge == nil {
		return "", ErrNotFound
	}

	isCorrect := challenge.Flag == flag
	if err := s.submissions.Create(ctx, &model.Submission{
		UserID:        userID,
		ChallengeID:   challengeID,
		SubmittedFlag: flag,
		IsCorrect:     isCorrect,
	}); err != nil {
		return "", err
	}

	if isCorrect {
		return ResultCorrect, nil
	}
	return ResultIncorrect, nil
}

func (s *SubmissionService) List(ctx context.Context, userID string, challengeID int) ([]model.Submission, error) {
	return s.submissions.List(ctx, userID, challengeID)
}

var _ = errors.New // keep import
```

---

## Task 5: JWT Middleware

- [ ] Create `internal/middleware/auth.go`

### internal/middleware/auth.go
```go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	CtxUserID contextKey = "user_id"
	CtxRole   contextKey = "role"
)

type Auth struct {
	secret    []byte
	adminRole string
}

func NewAuth(secret, adminRole string) *Auth {
	return &Auth{secret: []byte(secret), adminRole: adminRole}
}

func (a *Auth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return a.secret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
			return
		}
		userID, _ := claims["sub"].(string)
		role, _ := claims["role"].(string)
		ctx := context.WithValue(r.Context(), CtxUserID, userID)
		ctx = context.WithValue(ctx, CtxRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(CtxRole).(string)
		if role != a.adminRole {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserID(r *http.Request) string {
	v, _ := r.Context().Value(CtxUserID).(string)
	return v
}
```

---

## Task 6: Handlers

- [ ] Create `internal/handler/challenge.go`
- [ ] Create `internal/handler/submission.go`

### internal/handler/challenge.go
```go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ad7/internal/model"
	"ad7/internal/service"
)

type ChallengeHandler struct {
	svc *service.ChallengeService
}

func NewChallengeHandler(svc *service.ChallengeService) *ChallengeHandler {
	return &ChallengeHandler{svc: svc}
}

func (h *ChallengeHandler) List(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cs == nil {
		cs = []model.Challenge{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"challenges": cs})
}

func (h *ChallengeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	c, err := h.svc.Get(r.Context(), id)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *ChallengeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var c model.Challenge
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	id, err := h.svc.Create(r.Context(), &c)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *ChallengeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	var patch model.Challenge
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := h.svc.Update(r.Context(), id, &patch); err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ChallengeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseID(r *http.Request) int {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	return id
}
```

### internal/handler/submission.go
```go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
	"ad7/internal/service"
)

type SubmissionHandler struct {
	svc *service.SubmissionService
}

func NewSubmissionHandler(svc *service.SubmissionService) *SubmissionHandler {
	return &SubmissionHandler{svc: svc}
}

func (h *SubmissionHandler) Submit(w http.ResponseWriter, r *http.Request) {
	challengeID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var body struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Flag == "" {
		writeError(w, http.StatusBadRequest, "flag is required")
		return
	}
	userID := middleware.UserID(r)
	result, err := h.svc.Submit(r.Context(), userID, challengeID, body.Flag)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "challenge not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": result == service.ResultCorrect,
		"message": string(result),
	})
}

func (h *SubmissionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	challengeID, _ := strconv.Atoi(r.URL.Query().Get("challenge_id"))
	subs, err := h.svc.List(r.Context(), userID, challengeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"submissions": subs})
}
```

### internal/handler/util.go
```go
package handler

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

---

## Task 7: Main Entry Point

- [ ] Create `cmd/server/main.go`

### cmd/server/main.go
```go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"ad7/internal/config"
	"ad7/internal/handler"
	"ad7/internal/middleware"
	"ad7/internal/service"
	"ad7/internal/store"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	st, err := store.New(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer st.Close()

	auth := middleware.NewAuth(cfg.JWT.Secret, cfg.JWT.AdminRole)

	challengeSvc := service.NewChallengeService(st)
	submissionSvc := service.NewSubmissionService(st, st)

	challengeH := handler.NewChallengeHandler(challengeSvc)
	submissionH := handler.NewSubmissionHandler(submissionSvc)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
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

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
```

---

## Task 8: Smoke Test

- [ ] Apply schema
- [ ] Run server
- [ ] Test with curl

```bash
# Apply schema
mysql -u root < sql/schema.sql

# Build and run
go build -o ctf-server ./cmd/server && ./ctf-server

# Generate a test JWT (HS256, secret="change-me-in-production")
# Payload: {"sub":"user1","role":"admin","exp":9999999999}
# Use jwt.io or:
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMSIsInJvbGUiOiJhZG1pbiIsImV4cCI6OTk5OTk5OTk5OX0.placeholder"

# Create a challenge (admin)
curl -X POST http://localhost:8080/api/v1/admin/challenges \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Hello","category":"misc","description":"Find the flag","score":100,"flag":"flag{hello}"}'

# List challenges
curl http://localhost:8080/api/v1/challenges \
  -H "Authorization: Bearer $TOKEN"

# Submit flag
curl -X POST http://localhost:8080/api/v1/challenges/1/submit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"flag":"flag{hello}"}'
# Expected: {"message":"correct","success":true}

# Submit again (already solved)
curl -X POST http://localhost:8080/api/v1/challenges/1/submit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"flag":"flag{hello}"}'
# Expected: {"message":"already_solved","success":false}
```

---

## Notes

- `Store` implements both `ChallengeStore` and `SubmissionStore` interfaces — passed as `st` to both services in main.
- Flag is never returned in user-facing API responses (`json:"-"` on model).
- Admin endpoints require `role == cfg.JWT.AdminRole` in JWT claims.
- For smoke test JWT, generate a valid HS256 token at jwt.io with secret `change-me-in-production`.
