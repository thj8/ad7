# Standalone Auth Server Design

## Overview

Extract the auth module into a standalone HTTP server (`cmd/auth-server/`). The CTF project communicates with auth exclusively via HTTP — no direct auth package imports in CTF server code.

## Architecture

```
┌─────────────┐   HTTP    ┌──────────────┐
│  CTF Server  │ ────────→ │ Auth Server   │
│  :8080       │  verify   │ :8081         │
│              │           │               │
│ middleware/  │           │ /register     │
│ auth.go      │           │ /login        │
│ (calls       │           │ /verify       │
│  /verify)    │           │ /teams/*      │
└─────────────┘           └──────────────┘
       │                         │
       └──────┬──────────────────┘
              ▼
         MySQL (shared)
```

## New Files

| File | Purpose |
|------|---------|
| `cmd/auth-server/main.go` | Auth server entrypoint |
| `cmd/auth-server/config.yaml` | Auth server config (port, db, jwt) |
| `internal/auth/verify_handler.go` | `POST /api/v1/verify` handler |

## Modified Files

| File | Change |
|------|--------|
| `internal/middleware/auth.go` | `Authenticate` calls auth service `/verify` instead of local JWT parsing |
| `internal/config/config.go` | Add `auth.url` field |
| `cmd/server/main.go` | Remove all auth module code; CTF server no longer imports `internal/auth` |

## Auth Server API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/register` | No | Register user → `{id, username, role}` |
| POST | `/api/v1/login` | No | Login → `{token}` |
| POST | `/api/v1/verify` | No | Verify token → `{user_id, role}` (internal use) |
| GET | `/api/v1/teams` | JWT | List teams |
| GET | `/api/v1/teams/{id}` | JWT | Get team |
| GET | `/api/v1/teams/{id}/members` | JWT | List team members |
| POST | `/api/v1/admin/teams` | Admin | Create team |
| PUT | `/api/v1/admin/teams/{id}` | Admin | Update team |
| DELETE | `/api/v1/admin/teams/{id}` | Admin | Delete team |
| POST | `/api/v1/admin/teams/{id}/members` | Admin | Add member |
| DELETE | `/api/v1/admin/teams/{id}/members/{user_id}` | Admin | Remove member |

### Verify Endpoint

```
POST /api/v1/verify
Authorization: Bearer <token>

→ 200 { "user_id": "abc...", "role": "member" }
→ 401 { "error": "invalid token" }
```

This endpoint reuses the JWT secret to validate tokens. It's the single source of truth for token validation.

## CTF Middleware Change

`internal/middleware/auth.go` — `Authenticate` middleware:

- Before: parses and validates JWT locally
- After: calls `POST http://<auth.url>/api/v1/verify` with the Bearer token
- Extracts `user_id` and `role` from the verify response
- Injects into context as before (CtxUserID, CtxRole)

The `Auth` struct changes from holding a `secret` to holding an `authURL`:

```go
type Auth struct {
    authURL   string    // e.g. "http://localhost:8081"
    adminRole string
    client    *http.Client
}
```

## Config

### CTF config (`config.yaml`) — new field

```yaml
auth:
  url: "http://localhost:8081"  # auth service URL
```

### Auth server config (`cmd/auth-server/config.yaml`)

```yaml
server:
  port: 8081
db:
  host: "127.0.0.1"
  port: 3306
  user: "root"
  password: ""
  dbname: "ctf"
jwt:
  secret: "your-secret"
  admin_role: "admin"
log:
  level: "info"
```

## Database

Both servers connect to the same MySQL database. Auth server owns the `users` and `teams` tables. CTF server reads `user_id` from context (set by middleware after verify).

## Key Design Decisions

1. **Verify endpoint is unauthenticated** — it validates the token itself, no double-auth
2. **Shared database** — simplest approach for a single-deployment system
3. **Same Go module** — `cmd/auth-server/` lives in the same repo, imports `internal/auth/`
4. **http.Client with timeout** — CTF middleware uses a short-timeout HTTP client for verify calls
5. **RequireAdmin stays in CTF** — role check remains local since it's just a context value comparison
