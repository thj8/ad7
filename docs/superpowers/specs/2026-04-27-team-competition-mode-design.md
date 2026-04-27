# Team Competition Mode Design

Date: 2026-04-27

## Overview

Add team-based competition mode to the CTF platform. When one team member submits a correct flag, the team is credited. Leaderboard, first-blood, and analytics operate at team granularity in team mode. Individual mode remains completely unchanged.

## Requirements

| Decision | Conclusion |
|----------|------------|
| Enrollment | Two modes: free (any team member joins) + managed (admin adds teams) |
| Deduplication | Team-level: one correct submit = team solved |
| Submission records | Keep individual `user_id` for analytics |
| First blood | Team-level in team mode |
| Mode selection | Set at competition creation time, immutable |
| Individual mode | Completely unchanged |
| Access control | Challenge listing, leaderboard, analytics, notifications all gated by team membership in team mode |

## Data Model Changes

### competitions table

```sql
ALTER TABLE competitions ADD COLUMN mode VARCHAR(16) NOT NULL DEFAULT 'individual';
-- Values: 'individual' | 'team'
```

```sql
ALTER TABLE competitions ADD COLUMN team_join_mode VARCHAR(16) NOT NULL DEFAULT 'free';
-- Values: 'free' | 'managed', only meaningful when mode='team'
```

Existing competitions default to `mode=individual`, `team_join_mode=free`. No behavior change.

### competition_teams table (new)

```sql
CREATE TABLE competition_teams (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    res_id VARCHAR(32) NOT NULL UNIQUE,
    competition_id VARCHAR(32) NOT NULL,
    team_id VARCHAR(32) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    is_deleted TINYINT NOT NULL DEFAULT 0,
    UNIQUE INDEX idx_comp_team (competition_id, team_id, is_deleted)
);
```

Only used in managed mode. Free mode does not write to this table.

### submissions table

```sql
ALTER TABLE submissions ADD COLUMN team_id VARCHAR(32) DEFAULT NULL;
ALTER TABLE submissions ADD INDEX idx_team_chal_comp_correct (team_id, challenge_id, competition_id, is_correct);
```

- Individual mode: `team_id = NULL`, existing unique index `idx_user_chal_comp_correct` continues to work.
- Team mode: `team_id` set to user's team, deduplication uses new index.

### topthree_records table

```sql
ALTER TABLE topthree_records ADD COLUMN team_id VARCHAR(32) DEFAULT NULL;
```

Team mode first-blood records store `team_id`. Individual mode records remain `team_id = NULL`.

### Event struct

```go
type Event struct {
    Type          EventType
    UserID        string
    TeamID        string    // New: populated in team mode, empty in individual mode
    ChallengeID   string
    CompetitionID string
    SubmittedAt   time.Time
    Ctx           context.Context
}
```

## Submission Flow

### Individual mode

No changes. Existing logic path untouched.

### Team mode

1. Fetch competition, verify `mode == "team"`.
2. Call auth service `GET /api/v1/users/{id}/teams` to resolve user's team.
3. If no team -> 403 "must join a team to participate".
4. If `team_join_mode == "managed"` -> check `competition_teams`. If not registered -> 403 "your team is not registered for this competition".
5. Deduplication: `HasCorrectSubmission` queries by `(team_id, challenge_id, competition_id)` instead of `(user_id, ...)`.
6. Create submission with `team_id` populated, `user_id` still records actual submitter.
7. Publish `EventCorrectSubmission` with `TeamID` set.

## Access Control

### CheckCompAccess (shared service method)

```go
func (s *Service) CheckCompAccess(ctx context.Context, compID, userID string) error {
    comp := s.GetCompetition(ctx, compID)
    if comp.Mode == "individual" {
        return nil
    }
    team := s.ResolveUserTeam(ctx, userID)
    if team == nil {
        return ErrMustJoinTeam
    }
    if comp.TeamJoinMode == "managed" {
        if !s.store.IsTeamInComp(ctx, compID, team.ID) {
            return ErrTeamNotRegistered
        }
    }
    return nil
}
```

Admin role bypasses all checks.

### Gated endpoints

All endpoints below call `CheckCompAccess` before processing:

| Endpoint | Access rule in team mode |
|----------|--------------------------|
| `GET /competitions/{id}/challenges` | User must have team; managed: team must be in competition |
| `GET /competitions/{id}/challenges/{id}` | Same as above |
| `POST /competitions/{id}/challenges/{id}/submit` | Same as above |
| `GET /competitions/{id}/leaderboard` | Same as above |
| `GET /competitions/{id}/analytics/*` | Same as above |
| `GET /competitions/{id}/notifications` | Same as above |

## Leaderboard Plugin

### Individual mode

No changes.

### Team mode

- New `pluginutil` functions:
  - `GetTeamScores(ctx, db, compID) map[string]int` -- total score per team
  - `GetTeamSolveDetails(ctx, db, compID)` -- per-team per-challenge solve time
- Leaderboard entry: `rank`, `team_id`, `team_name`, `total_score`, `last_solve_time`, per-challenge detail
- Sort: total score desc, ties broken by last solve time asc (same rule as individual)

## TopThree (First Blood) Plugin

### Individual mode

No changes.

### Team mode

- Event handler: when `TeamID != ""`, use `team_id` for deduplication in `topthree_records`.
- `TopThreeProvider` interface additions:
  - `GetTeamBloodRank(ctx, compID, chalID, teamID) int`
  - `GetCompTeamTopThree(ctx, compID) map[string]BloodRankEntry`
- Leaderboard calls team-version API in team mode.

## Analytics Plugin

### Individual mode

No changes. Existing four endpoints work as-is.

### Team mode additions

Two new endpoints:

- `GET /api/v1/competitions/{id}/analytics/teams` -- team-level stats: `team_id`, `team_name`, `total_solves`, `total_score`, `total_attempts`, `success_rate`, `first_solve_time`, `last_solve_time`
- `GET /api/v1/competitions/{id}/analytics/teams/{team_id}/members` -- per-member stats within a team: `user_id`, `username`, `total_solves`, `total_score`, `total_attempts`, `success_rate`

Existing endpoints remain accessible in team mode (individual data is preserved in submissions).

## Management API

### Create competition

`POST /api/v1/admin/competitions` body additions:

```json
{
  "mode": "team",
  "team_join_mode": "free"
}
```

- `mode`: `individual` (default) | `team`
- `team_join_mode`: `free` (default) | `managed`, only effective when mode=team

### Team management (managed mode only)

```
POST   /api/v1/admin/competitions/{id}/teams              -- Add team
DELETE /api/v1/admin/competitions/{id}/teams/{team_id}     -- Remove team
GET    /api/v1/competitions/{id}/teams                     -- List teams in competition
```

Validation:
- Returns 400 if competition is not in team mode
- Returns 400 if competition uses free join mode

## Store Interface Additions

`CompetitionStore` adds:

- `AddCompTeam(ctx, compID, teamID) error`
- `RemoveCompTeam(ctx, compID, teamID) error`
- `ListCompTeams(ctx, compID) ([]TeamEntry, error)`
- `IsTeamInComp(ctx, compID, teamID) (bool, error)`

`SubmissionStore` adds:

- `HasTeamCorrectSubmission(ctx, teamID, chalID, compID) (bool, error)`

## Team Resolution

CTF service resolves user's team by calling auth service:

- `GET /api/v1/users/{id}/teams` (existing auth endpoint)
- Encapsulated in a `TeamResolver` at the service layer
- Returns user's current team or nil

## Error Handling

| Scenario | HTTP Status | Error Message |
|----------|-------------|---------------|
| Team-mode user has no team | 403 | "must join a team to participate" |
| Managed mode, team not registered | 403 | "your team is not registered for this competition" |
| Team already solved challenge | 409 | "your team already solved this challenge" |
| Non-team comp, team management API called | 400 | "competition is not in team mode" |
| Free mode, admin tries to add teams | 400 | "competition uses free join mode" |
| Invalid mode/team_join_mode value | 400 | "invalid mode value" |

## Testing Plan

### Integration tests (`internal/integration/`)

1. **Free mode**:
   - User with team submits correct flag -> team scored
   - Another member of same team submits same challenge -> rejected (team already solved)
   - User without team submits -> 403

2. **Managed mode**:
   - Admin adds/removes teams
   - Added team member submits -> success
   - Non-added team member submits -> 403
   - After removal, member submits -> 403

3. **Leaderboard**:
   - Team mode ranks by team
   - Individual mode unchanged

4. **First blood**:
   - Team mode first blood by team
   - Individual mode unchanged

5. **Analytics**:
   - Team-level stats correct
   - Within-team member stats correct

6. **Mode isolation**:
   - Individual mode behavior completely unaffected
   - Team mode individual data still queryable

7. **Access control**:
   - Team mode: challenge listing, leaderboard gated by team membership
   - Managed mode: gated by competition_teams
   - Admin bypasses all checks

## Files Changed (Summary)

| Layer | Files |
|-------|-------|
| Model | `internal/model/competition.go` (add Mode, TeamJoinMode) |
| Store | `internal/store/store.go` (new interface methods), `internal/store/mysql.go` (implementations) |
| Service | `internal/service/submission.go` (team dedup), `internal/service/competition.go` (team management, access check) |
| Handler | `internal/handler/competition.go` (team CRUD endpoints), `internal/handler/submission.go` (team flow) |
| Router | `internal/router/competitions.go` (new routes) |
| Event | `internal/event/event.go` (TeamID field) |
| Plugin util | `internal/pluginutil/queries.go` (team query functions) |
| Plugin: leaderboard | `plugins/leaderboard/leaderboard.go` (team ranking) |
| Plugin: topthree | `plugins/topthree/*.go` (team first blood) |
| Plugin: analytics | `plugins/analytics/analytics.go` (team analytics endpoints) |
| SQL | `sql/migrations/002_team_competition_mode.sql` |
| Tests | `internal/integration/team_competition_test.go` |
