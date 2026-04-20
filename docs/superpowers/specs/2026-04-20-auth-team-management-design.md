# Auth & Team Management Design

## Overview

Add user registration/login and team CRUD under `internal/auth/`. Users belong to teams (many-to-one). Passwords stored with bcrypt. JWT tokens issued on login.

## Data Model

### users table

```sql
CREATE TABLE IF NOT EXISTS users (
    id            INT AUTO_INCREMENT PRIMARY KEY,
    res_id        VARCHAR(32)   NOT NULL UNIQUE,
    username      VARCHAR(255)  NOT NULL UNIQUE,
    password_hash VARCHAR(255)  NOT NULL,
    role          VARCHAR(64)   NOT NULL DEFAULT 'member',
    team_id       VARCHAR(32)   DEFAULT NULL,
    is_deleted    TINYINT(1)    NOT NULL DEFAULT 0,
    created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_team (team_id)
);
```

### teams table

```sql
CREATE TABLE IF NOT EXISTS teams (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    res_id      VARCHAR(32)   NOT NULL UNIQUE,
    name        VARCHAR(255)  NOT NULL,
    description VARCHAR(4096) NOT NULL DEFAULT '',
    is_deleted  TINYINT(1)    NOT NULL DEFAULT 0,
    created_at  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

## API Endpoints

### Public (no auth required)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/register` | Register user |
| POST | `/api/v1/login` | Login, returns JWT |

### Authenticated (JWT required)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/teams` | List teams |
| GET | `/api/v1/teams/{id}` | Get team detail |
| GET | `/api/v1/teams/{id}/members` | List team members |

### Admin (JWT + admin role)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/admin/teams` | Create team |
| PUT | `/api/v1/admin/teams/{id}` | Update team |
| DELETE | `/api/v1/admin/teams/{id}` | Delete team |
| POST | `/api/v1/admin/teams/{id}/members` | Add user to team |
| DELETE | `/api/v1/admin/teams/{id}/members/{user_id}` | Remove user from team |

## File Structure

```
internal/auth/
├── model.go       # User, Team models (embed BaseModel)
├── store.go       # UserStore, TeamStore interfaces
├── mysql.go       # MySQL implementations
├── service.go     # AuthService (Register, Login, GenerateToken)
├── team_service.go # TeamService (CRUD + member management)
├── handler.go     # AuthHandler (Register, Login)
├── team_handler.go # TeamHandler (CRUD + members)
├── router.go      # RegisterAuthRoutes, RegisterAdminTeamRoutes
```

Plus modifications:
- `sql/schema.sql` — add users, teams tables
- `internal/store/store.go` — no change (auth has own store)
- `internal/config/config.go` — add JWT expiry config
- `cmd/server/main.go` — wire auth module

## Key Design Decisions

1. **Separate package**: `internal/auth/` owns its store, service, handler, and router — same layered pattern but self-contained
2. **JWT generation in service**: `AuthService.GenerateToken(userID, role)` uses the configured secret
3. **Role**: default `member`, admin role matches config `jwt.admin_role`
4. **user_id in submissions**: existing `user_id VARCHAR(128)` stores the user's `res_id`
5. **Team membership**: tracked via `users.team_id` FK (no join table needed for many-to-one)

## Request/Response Shapes

### POST /api/v1/register
```json
// Request
{ "username": "player1", "password": "secret123" }
// Response 201
{ "id": "abc...", "username": "player1", "role": "member" }
```

### POST /api/v1/login
```json
// Request
{ "username": "player1", "password": "secret123" }
// Response 200
{ "token": "eyJhbGci..." }
```

### POST /api/v1/admin/teams
```json
// Request
{ "name": "Team Alpha", "description": "..." }
// Response 201
{ "id": "xyz..." }
```

### POST /api/v1/admin/teams/{id}/members
```json
// Request
{ "user_id": "abc..." }
// Response 200
{ "message": "ok" }
```
