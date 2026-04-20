# Test Distribution Design

Date: 2026-04-20

## Goal

Distribute integration tests from the monolithic `internal/integration/integration_test.go` into each plugin's own directory, add missing test coverage, and enable `go test ./plugins/xxx/...` to run independently.

## Approach

**Shared testutil + per-directory tests** (Plan B).

Create `internal/testutil/` package with shared setup and helpers. Each plugin directory gets its own `_test.go` file with an independent `TestMain`.

## New Files

```
internal/testutil/
  testutil.go            # TestEnv struct, NewTestEnv, Cleanup, helpers

plugins/leaderboard/
  leaderboard_test.go    # TestCompetitionLeaderboard

plugins/notification/
  notification_test.go   # TestCompetitionNotifications + Update/Delete

plugins/analytics/
  analytics_test.go      # TestAnalyticsOverview, Categories, Users, Challenges

plugins/hints/
  hints_test.go          # TestHints (CRUD + visible toggle + 404)

plugins/topthree/
  topthree_test.go       # TestTopThree + BaseModel soft delete
```

## internal/testutil/testutil.go

### TestEnv struct

```go
type TestEnv struct {
    Server *httptest.Server
    DB     *sql.DB
}
```

### NewTestEnv(t *testing.M) *TestEnv

- Read `TEST_DSN` from env (fallback to default DSN)
- Create store, wire services/handlers/plugins
- Build full chi router with all routes
- Return TestEnv with httptest.Server

### Cleanup(t *testing.T)

Clear tables in dependency order: topthree_records, hints, competition_challenges, notifications, submissions, competitions, challenges.

### Helpers

- `MakeToken(userID, role string) string` — sign JWT
- `DoRequest(t, method, path, body, token string) *http.Response`
- `DecodeJSON(t *testing.T, r *http.Response) map[string]any`
- `GetID(t *testing.T, m map[string]any) string`
- `AssertStatus(t *testing.T, resp *http.Response, want int)`

## Modified Files

### internal/integration/integration_test.go

Reduced to core tests only:

| Test | Coverage |
|------|----------|
| TestListChallenges | GET /challenges, flag not leaked |
| TestGetChallenge | GET /challenges/{id}, 404, 401 |
| TestAdminCreateChallenge | POST, 403, 401 |
| TestAdminUpdateChallenge | PUT, 404, 403, 401 |
| TestAdminDeleteChallenge | DELETE, soft delete verify, 403, 401 |
| TestCompetitions | Full CRUD + 403, 400 |
| TestCompetitionChallenges | Add/remove challenge, flag not leaked |
| TestSubmitInCompetition | Correct/wrong/already_solved, 401 |
| TestAdminListSubmissions | List + filter by user_id/challenge_id |
| TestSubmitFlagRateLimit | 429 after 3 requests, different user ok |
| TestCompetitionStartEnd | Start/End/repeat 409/404/403 |
| TestCompetitionAutoStatus | Auto activate, auto end, ListActive filter |
| TestInputValidation (NEW) | Oversized title/flag/description, empty body, invalid JSON |
| TestJWTExpiry (NEW) | Expired token returns 401 |
| TestAdminListAllCompetitions (NEW) | GET /admin/competitions returns all (including inactive) |

### TestMain in each plugin

Each plugin's `_test.go` calls `testutil.NewTestEnv` and `defer env.Close()`. No shared state between plugin test packages.

## New Test Cases

| Location | Test | What it verifies |
|----------|------|------------------|
| plugins/analytics/ | TestAnalyticsUsers | /competitions/{id}/analytics/users returns per-user stats |
| plugins/analytics/ | TestAnalyticsChallenges | /competitions/{id}/analytics/challenges returns per-challenge stats |
| plugins/notification/ | TestNotificationUpdate | PUT update notification content |
| plugins/notification/ | TestNotificationDelete | DELETE notification, verify removed |
| internal/integration/ | TestInputValidation | 400 for title>255, flag>255, desc>4096, empty body, invalid JSON |
| internal/integration/ | TestJWTExpiry | Expired token → 401 |
| internal/integration/ | TestAdminListAllCompetitions | Admin sees all competitions including inactive ones |

## Migration Steps

1. Create `internal/testutil/testutil.go` with shared setup + helpers
2. Move plugin tests to respective directories
3. Slim down `internal/integration/integration_test.go` to core only
4. Add missing test cases
5. Verify all tests pass independently and as a whole

## Constraints

- Each test package runs independently: `go test ./plugins/leaderboard/...`
- All tests pass with `go test ./...`
- Integration tests require MySQL (same as before)
- TEST_DSN env var for database connection
