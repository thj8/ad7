# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run server (from project root)
go run ./cmd/server -config config.yaml

# Seed test data (15 competitions, 50 challenges, 30 users each)
go run ./cmd/seed/
TEST_DSN="root:pass@tcp(host:3306)/ctf?parseTime=true" go run ./cmd/seed/

# Demo script (query competitions, submit flags, check leaderboard)
./scripts/demo.sh
BASE_URL=http://host:8080 ./scripts/demo.sh

# Run all tests
go test ./...

# Run integration tests only (requires MySQL at 192.168.5.44)
go test ./internal/integration/... -v -count=1

# Run a single test
go test ./internal/integration/... -v -run TestSubmitFlag -count=1

# Apply schema to DB
mysql -h 192.168.5.44 -u root -pasfdsfedarjeiowvgfsd ctf < sql/schema.sql
```

## Architecture

Layered Go service: **handler → service → store** + **plugin system**

- `cmd/server/main.go` — wires everything: loads config, opens DB, creates store/services/handlers, registers chi routes, loads plugins, starts HTTP server
- `cmd/seed/main.go` — populates DB with test data: 50 challenges, 15 competitions, 30 users per competition with differentiated solve rates (top user 72%)
- `internal/config/` — YAML config loading (`server.port`, `db.*`, `jwt.secret`, `jwt.admin_role`)
- `internal/model/` — domain structs: `Challenge`, `Submission`, `Notification`, `Competition`, `CompetitionChallenge`. `Flag` has `json:"-"` so it never appears in API responses. All entities use snowflake `res_id` (int64) as public ID.
- `internal/store/` — `store.go` defines `ChallengeStore`, `SubmissionStore`, `CompetitionStore` interfaces; `mysql.go` implements all on a single `*Store` struct
- `internal/service/` — business logic: `ChallengeService` (CRUD), `SubmissionService` (flag verification, in-competition submission), `CompetitionService` (competition CRUD, challenge assignment)
- `internal/handler/` — HTTP layer only; uses separate request structs to receive fields that have `json:"-"` on models
- `internal/middleware/` — JWT auth (`Authenticate`) and admin gate (`RequireAdmin`); extracts `sub`→`user_id` and `role` from claims into context
- `internal/plugin/` — `Plugin` interface: `Register(r chi.Router, db *sql.DB, auth *middleware.Auth)`
- `internal/snowflake/` — snowflake ID generator (41-bit timestamp + 10-bit machine + 12-bit sequence)
- `plugins/leaderboard/` — per-competition leaderboard, ranked by total score desc, ties by earliest solve time asc
- `plugins/notification/` — per-competition notifications, admin creates, all users can list

## Key Design Decisions

**Snowflake res_id**: All entities use `res_id BIGINT` (snowflake) as the public-facing ID. The auto-increment `id` column is internal only (`json:"-"`). API paths and responses use `res_id` exclusively.

**Flag field**: `model.Challenge.Flag` is `json:"-"`. Handlers use separate request structs (`createRequest`, `updateRequest`) to decode the flag from incoming JSON, then manually assign it to the model before passing to the service.

**Single store struct**: `*store.Store` implements all store interfaces. In `main.go` it is passed to all services.

**Admin auth**: JWT middleware reads `role` claim; `RequireAdmin` compares it to `cfg.JWT.AdminRole` (default `"admin"`). User identity comes from `sub` claim.

**Competition-scoped**: There is no global leaderboard or global notifications. Everything is scoped to a competition. Submissions outside competitions are still supported for backwards compatibility.

**Plugin system**: Plugins implement the `Plugin` interface and register their own chi routes in `main.go`. They receive `*sql.DB` and `*middleware.Auth` for direct DB access and route protection.

**No foreign keys**: Per project constraint, the database does not use foreign key constraints.

**Input validation**: String fields have length limits (title/flag max 255, description max 4096). `parseID` returns 400 on invalid input. Notification `message` is required.

## Integration Tests

Tests in `internal/integration/` connect to a real MySQL instance. 12 tests covering:
- Challenge CRUD (List, Get, Submit, Admin CRUD, Submissions list)
- Competition CRUD
- Competition challenge assignment
- In-competition flag submission
- Per-competition leaderboard
- Per-competition notifications

`testDSN` reads from `TEST_DSN` env var, falls back to hardcoded default. `testSecret` is separate from production secret.

Each test calls `cleanup(t)` which deletes from all tables in dependency order.

## 约束
- 每次添加新功能，都必须添加完成的测试用例
- 改一个bug，要写一个测试用例，保证此bug不会再次发生
- 数据库不要使用外键关联