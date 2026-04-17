# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run server (from project root)
go run ./cmd/server -config config.yaml

# Run all tests
go test ./...

# Run integration tests only (requires MySQL at 192.168.5.44)
go test ./internal/integration/... -v -count=1

# Run a single test
go test ./internal/integration/... -v -run TestSubmitFlag -count=1
```

## Architecture

Layered Go service: **handler → service → store**

- `cmd/server/main.go` — wires everything: loads config, opens DB, creates store/services/handlers, registers chi routes, starts HTTP server
- `internal/config/` — YAML config loading (`server.port`, `db.*`, `jwt.secret`, `jwt.admin_role`)
- `internal/model/` — domain structs only (`Challenge`, `Submission`). `Flag` has `json:"-"` so it never appears in API responses
- `internal/store/` — `store.go` defines `ChallengeStore` and `SubmissionStore` interfaces; `mysql.go` implements both on a single `*Store` struct
- `internal/service/` — business logic: `ChallengeService` (CRUD with validation), `SubmissionService` (flag verification, duplicate-solve prevention)
- `internal/handler/` — HTTP layer only; uses `createRequest`/`updateRequest` structs (not `model.Challenge`) to receive `flag` from JSON since the model field is `json:"-"`
- `internal/middleware/` — JWT auth (`Authenticate`) and admin gate (`RequireAdmin`); extracts `sub`→`user_id` and `role` from claims into context

## Key Design Decisions

**Flag field**: `model.Challenge.Flag` is `json:"-"`. Handlers use separate request structs (`createRequest`, `updateRequest`) to decode the flag from incoming JSON, then manually assign it to the model before passing to the service.

**Single store struct**: `*store.Store` implements both `ChallengeStore` and `SubmissionStore`. In `main.go` it is passed as `st` to both `NewChallengeService(st)` and `NewSubmissionService(st, st)`.

**Admin auth**: JWT middleware reads `role` claim; `RequireAdmin` compares it to `cfg.JWT.AdminRole` (default `"admin"`). User identity comes from `sub` claim.

**Partial update**: `service.Update` fetches the existing record, applies non-zero patch fields, then calls `store.Update` which does a full overwrite. PUT requests must include `flag` and `is_enabled` explicitly.

## Integration Tests

Tests in `internal/integration/` connect to a real MySQL instance. Constants at the top of the file:
- `testDSN` — MySQL connection string
- `testSecret` — JWT signing secret used by the test router (different from production secret)

Each test calls `cleanup(t)` which deletes all submissions then all challenges (order matters due to FK constraint).


## 约束
- 每次添加新功能，都必须添加完成的测试用例
- 改一个bug，要写一个测试用例，保证此bug不会再次发生
- 数据库不要使用外键关联
