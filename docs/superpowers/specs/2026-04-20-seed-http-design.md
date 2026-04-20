# Seed HTTP Migration Design

## Overview

Convert `cmd/seed/main.go` from direct SQL inserts to HTTP API calls. This ensures submissions traverse the full stack (service → event → plugin), enabling testing of topthree (一血/二血/三血) recording on correct flag submissions.

## Architecture

```
seed CLI ──HTTP──→ Auth Server (:8081)   register, login
seed CLI ──HTTP──→ CTF Server (:8080)    challenges, competitions, submit
```

Sequential execution. No concurrency. No direct database access.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_URL` | `http://localhost:8081` | Auth server base URL |
| `CTF_URL` | `http://localhost:8080` | CTF server base URL |

Removes `TEST_DSN` dependency entirely.

## Execution Flow

1. Read `AUTH_URL` and `CTF_URL` from environment
2. Register admin user (`seed_admin`) via `POST /api/v1/register`
3. Login admin via `POST /api/v1/login` → obtain admin JWT
4. Create 50 challenges via `POST /api/v1/admin/challenges` → store `{res_id: flag}` map
5. For each of 15 competitions:
   - Create competition via `POST /api/v1/admin/competitions`
   - Assign 25 challenges via `POST /api/v1/admin/competitions/{id}/challenges`
   - Start competition via `POST /api/v1/admin/competitions/{id}/start`
   - For each of 30 users:
     - Register via `POST /api/v1/register`
     - Login via `POST /api/v1/login` → obtain user JWT
     - Submit flags via `POST /api/v1/competitions/{comp_id}/challenges/{id}/submit` per solveCounts schedule
   - End competition via `POST /api/v1/admin/competitions/{id}/end`

## Key Changes

**Remove:**
- `database/sql` import and all DB operations
- `TEST_DSN` environment variable
- `-clean` flag and `cleanAll()` function
- `uuid` import (IDs generated server-side)
- `dsn()`, `createChallenges()`, `createComp()`, `assignChals()`, `genSubmissions()`, `insertSub()`

**Add:**
- HTTP client helper functions: `postJSON()`, `getToken()`
- API call functions: `registerAndLogin()`, `createChallenge()`, `createCompetition()`, `addChallengeToComp()`, `startComp()`, `endComp()`, `submitFlag()`

**Keep unchanged:**
- All template data (competition titles, challenge templates, solveCounts, categories, scores)
- `pickN()` random selection helper
- Overall flow structure (15 competitions × 25 challenges × 30 users)

## Submission Timing

The submission endpoint is rate-limited at 3 requests/10 seconds per user. Sequential execution naturally spaces requests:
- Each user makes 1-18 correct + 0-5 incorrect submissions
- Between HTTP round-trips (~50ms local), sequential timing stays well within rate limits
- No backoff or retry logic needed

## Error Handling

- Any non-2xx API response → `log.Fatalf` with status code and response body
- HTTP connection errors → `log.Fatalf`
- Registration of existing user (409) → login instead (idempotent re-runs)

## File Changes

| File | Change |
|------|--------|
| `cmd/seed/main.go` | Complete rewrite: replace SQL with HTTP calls |

Single file change. No new files.
