# User-Team Relationship Redesign

**Date:** 2026-04-21
**Status:** Draft

## Background

Current implementation uses a `team_id` column on the `users` table to represent user-team membership (many-to-one). This limits users to a single team and makes future multi-team support difficult.

## Goal

Replace the direct `users.team_id` column with a dedicated `team_members` association table, supporting team-internal roles (captain/member). The first version enforces one-user-one-team via service layer; future versions can relax this constraint.

**Scope:** Auth service only. CTF main server is unaffected.

## Data Model

### New Table: `team_members`

```sql
CREATE TABLE team_members (
    id INT AUTO_INCREMENT PRIMARY KEY,
    res_id VARCHAR(32) NOT NULL UNIQUE,
    team_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    role VARCHAR(64) NOT NULL DEFAULT 'member',
    is_deleted TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_team (team_id),
    INDEX idx_user (user_id),
    UNIQUE INDEX idx_team_user_active (team_id, user_id, is_deleted)
);
```

- Follows BaseModel convention (id, res_id, created_at, updated_at, is_deleted)
- `role`: `captain` or `member`
- Unique index prevents duplicate active memberships per team

### New Go Model: `TeamMember`

```go
type TeamMember struct {
    model.BaseModel
    TeamID string `json:"team_id"`
    UserID string `json:"user_id"`
    Role   string `json:"role"`
}
```

### User Model Change

- Remove `TeamID` field from `User` struct
- Remove `team_id` column and `idx_team` index from `users` table

## Store Layer

### Removed

- `UserStore.SetTeamID`
- `UserStore.ListUsersByTeam`

### New Interface: `TeamMemberStore`

```go
type TeamMemberStore interface {
    AddMember(ctx context.Context, teamID, userID, role string) (*TeamMember, error)
    RemoveMember(ctx context.Context, teamID, userID string) error
    GetMember(ctx context.Context, teamID, userID string) (*TeamMember, error)
    ListTeamMembers(ctx context.Context, teamID string) ([]*TeamMember, error)
    GetUserTeams(ctx context.Context, userID string) ([]*TeamMember, error)
    GetTeamMemberCount(ctx context.Context, teamID string) (int, error)
}
```

### TeamStore Change

- `DeleteTeam` soft-deletes all `team_members` records for the team instead of setting `users.team_id = NULL`

### UserStore Change

- Add `ListUsersByResIDs(ctx, []string) ([]*User, error)` for batch user lookups

## Service Layer

### TeamService Changes

| Method | Behavior |
|--------|----------|
| `AddMember(ctx, teamID, userID)` | Check single-team constraint via `GetUserTeams`, then create TeamMember with default role `member` |
| `RemoveMember(ctx, teamID, userID)` | Enforce captain protection (cannot remove if other members exist) |
| `ListMembers(ctx, teamID)` | Return `[]MemberInfo` with username + role + joined_at |
| `SetCaptain(ctx, teamID, userID)` | Promote existing member to captain, demote current captain to member |
| `TransferCaptain(ctx, teamID, fromUserID, toUserID)` | Explicit captain transfer |

### MemberInfo Response Struct

```go
type MemberInfo struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    JoinedAt string `json:"joined_at"`
}
```

### Business Rules

| Rule | Detail |
|------|--------|
| Single-team constraint | `AddMember` rejects if user already belongs to another team |
| Captain uniqueness | One captain per team |
| Captain removal protection | Cannot remove captain while other members exist; must transfer first |
| Delete cascade | `DeleteTeam` soft-deletes team + all associated `team_members` |
| Auto-captain on create | Team creator automatically becomes captain |

### Unchanged

- AuthService (register, login, JWT) unchanged
- Team CRUD (Create, Update, Get, List) unchanged

## API Endpoints

### Modified

| Endpoint | Change |
|----------|--------|
| `POST /api/v1/admin/teams/{id}/members` | Body: `{"user_id":"...", "role":"member"}` (role optional, defaults to `member`) |
| `GET /api/v1/teams/{id}/members` | Returns `MemberInfo[]` with username, role, joined_at |

### New

| Endpoint | Method | Body |
|----------|--------|------|
| `PUT /api/v1/admin/teams/{id}/captain` | SetCaptain | `{"user_id":"..."}` |
| `POST /api/v1/admin/teams/{id}/transfer-captain` | TransferCaptain | `{"to_user_id":"..."}` |

### Error Responses

| Scenario | Status | Message |
|----------|--------|---------|
| User already in another team | 409 | `user already belongs to another team` |
| User already a member | 409 | `user is already a member of this team` |
| Remove captain with members | 400 | `cannot remove captain, transfer captain first` |
| Target not a member | 400 | `user is not a member of this team` |
| Team not found | 404 | `team not found` |
| User not found | 404 | `user not found` |

## Data Migration

```sql
-- 1. Create team_members table
CREATE TABLE team_members (...);

-- 2. Migrate existing users.team_id → team_members
INSERT INTO team_members (res_id, team_id, user_id, role, is_deleted, created_at, updated_at)
SELECT
    REPLACE(UUID(), '-', '') AS res_id,
    u.team_id,
    u.res_id AS user_id,
    CASE WHEN u.id = (
        SELECT MIN(u2.id) FROM users u2
        WHERE u2.team_id = u.team_id AND u2.is_deleted = 0
    ) THEN 'captain' ELSE 'member' END AS role,
    0 AS is_deleted,
    u.created_at,
    u.updated_at
FROM users u
WHERE u.team_id IS NOT NULL AND u.team_id != '' AND u.is_deleted = 0;

-- 3. Drop old column
ALTER TABLE users DROP INDEX idx_team;
ALTER TABLE users DROP COLUMN team_id;
```

Captain assignment: earliest-registered user (lowest `id`) per team becomes captain.

## Testing

| Type | Coverage |
|------|----------|
| Unit tests | TeamMemberStore CRUD, unique constraint, soft delete |
| Integration tests | AddMember single-team constraint, RemoveMember captain protection, SetCaptain, TransferCaptain, DeleteTeam cascade |
| Existing test adaptation | Update testutil mock data referencing `team_id` |

## Unchanged Components

- CTF main server (`cmd/server`) — no awareness of teams
- Middleware — no team involvement
- Plugin system — leaderboard, notifications, etc. unchanged
- `pluginutil` shared queries — unchanged
- AuthService (register/login/JWT) — unchanged

## Future Expansion

To support multi-team membership:
1. Remove single-team constraint check in `AddMember`
2. Optionally add per-competition team context
3. Add team-switching API for users
