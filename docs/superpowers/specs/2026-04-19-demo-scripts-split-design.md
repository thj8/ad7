# Demo Scripts Split Design

## Goal

Rewrite `scripts/demo.sh` into independent per-resource test scripts for convenient API testing.

## Structure

```
scripts/
  test-competitions.sh   ← 比赛 CRUD + 开始/结束
  test-challenges.sh     ← 题目 CRUD
  test-submissions.sh    ← Flag 提交 + 记录查询
  test-leaderboard.sh    ← 排行榜
  test-notifications.sh  ← 通知 CRUD
  test-hints.sh          ← 提示 CRUD + 列表
  test-analytics.sh      ← 分析 4 个端点
  demo.sh                ← 一键跑全部（调用其他脚本）
```

No shared `common.sh` — each script is fully independent, containing its own JWT generator, header setup, and JSON pretty-printer. This means each script runs standalone with no dependencies on other scripts.

## Conventions

- Each script prints clear section headers (`=== Section ===`)
- Errors are printed, not silently skipped
- Scripts auto-query IDs they need via API calls + python3 JSON parsing
- `BASE_URL` env var for server address (default `http://localhost:8080`)
- `JWT_SECRET` env var for JWT secret (default `change-me-in-production`)

## Endpoint Coverage Per Script

### test-competitions.sh

| Method | Path | Notes |
|--------|------|-------|
| GET | `/api/v1/competitions` | List active competitions (user) |
| GET | `/api/v1/competitions/{id}` | Get competition detail |
| GET | `/api/v1/competitions/{id}/challenges` | List challenges in competition |
| GET | `/api/v1/admin/competitions` | List all competitions (admin) |
| POST | `/api/v1/admin/competitions` | Create competition |
| PUT | `/api/v1/admin/competitions/{id}` | Update competition |
| DELETE | `/api/v1/admin/competitions/{id}` | Delete competition |
| POST | `/api/v1/admin/competitions/{id}/start` | Start competition |
| POST | `/api/v1/admin/competitions/{id}/end` | End competition |
| POST | `/api/v1/admin/competitions/{id}/challenges` | Add challenge to competition |
| DELETE | `/api/v1/admin/competitions/{id}/challenges/{cid}` | Remove challenge from competition |

Creates a test competition, tests all operations, cleans up by deleting it.

### test-challenges.sh

| Method | Path | Notes |
|--------|------|-------|
| GET | `/api/v1/challenges` | List challenges |
| GET | `/api/v1/challenges/{id}` | Get challenge detail |
| POST | `/api/v1/admin/challenges` | Create challenge |
| PUT | `/api/v1/admin/challenges/{id}` | Update challenge |
| DELETE | `/api/v1/admin/challenges/{id}` | Delete challenge |

Creates a test challenge, tests all operations, cleans up.

### test-submissions.sh

| Method | Path | Notes |
|--------|------|-------|
| POST | `/api/v1/competitions/{id}/challenges/{cid}/submit` | Submit flag |
| GET | `/api/v1/admin/competitions/{id}/submissions` | List submissions |

Auto-picks first competition + first challenge. Submits wrong flag, then correct flag (queries DB for real flag).

### test-leaderboard.sh

| Method | Path | Notes |
|--------|------|-------|
| GET | `/api/v1/competitions/{id}/leaderboard` | Get leaderboard |

Auto-picks first competition.

### test-notifications.sh

| Method | Path | Notes |
|--------|------|-------|
| POST | `/api/v1/admin/competitions/{id}/notifications` | Create notification |
| GET | `/api/v1/competitions/{id}/notifications` | List notifications |

Creates a test notification, lists it, cleans up.

### test-hints.sh

| Method | Path | Notes |
|--------|------|-------|
| POST | `/api/v1/admin/challenges/{id}/hints` | Create hint |
| PUT | `/api/v1/admin/hints/{id}` | Update hint |
| DELETE | `/api/v1/admin/hints/{id}` | Delete hint |
| GET | `/api/v1/challenges/{id}/hints` | List hints |

Creates a test hint on first challenge, tests all operations, cleans up.

### test-analytics.sh

| Method | Path | Notes |
|--------|------|-------|
| GET | `/api/v1/competitions/{id}/analytics/overview` | Overview stats |
| GET | `/api/v1/competitions/{id}/analytics/categories` | Category breakdown |
| GET | `/api/v1/competitions/{id}/analytics/users` | User stats |
| GET | `/api/v1/competitions/{id}/analytics/challenges` | Challenge stats |

Auto-picks first competition.

### demo.sh

Calls all `test-*.sh` scripts in sequence. Acts as a full smoke test.

## Dependency Data Strategy

- Scripts that need to create data (competitions, challenges, notifications, hints) create their own test entities and clean up after
- Scripts that only read data (leaderboard, analytics, submissions) auto-query the first available entity
- `test-submissions.sh` queries MySQL for the real flag (same approach as current demo.sh)

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `BASE_URL` | `http://localhost:8080` | Server address |
| `JWT_SECRET` | `change-me-in-production` | JWT signing secret |
