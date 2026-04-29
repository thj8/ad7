<!-- Generated: 2026-04-27 | Files scanned: 6 | Token estimate: ~400 -->

# Dependencies

## External Services
- **MySQL** (primary data store)
  - Shared by both CTF server and Auth server
  - Requires TEST_DSN env var for integration tests

## Go Dependencies
```
github.com/go-chi/chi/v5          (HTTP router)
github.com/go-sql-driver/mysql    (MySQL driver)
github.com/golang-jwt/jwt/v5      (JWT tokens)
github.com/labstack/echo/v4       (HTTP framework, auth server)
github.com/redis/go-redis/v9      (rate limiting)
github.com/stretchr/testify       (testing)
golang.org/x/crypto               (password hashing)
log/slog                          (structured logging, stdlib)
```

## Internal Dependencies
```
CTF Server → Auth Server (HTTP: /api/v1/verify)
Plugins → pluginutil (shared queries)
Leaderboard → TopThreeProvider (interface dependency)
TopThree → EventBus (subscribes to EventCorrectSubmission)
```

## Configuration
- YAML config via `config.yaml`
- Server port, DB connection, auth URL, JWT settings, log settings, rate limit settings
- Env vars: TEST_DSN, BASE_URL, JWT_SECRET

## Build & Test
```
go build ./...              (build all)
go test ./... -short        (unit tests only)
go test ./internal/integration/...  (integration tests, needs MySQL)
```
