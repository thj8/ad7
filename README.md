# AD7 - CTF Jeopardy Platform

A multi-competition CTF (Capture The Flag) Jeopardy platform built with Go.

## Features

- **Multi-competition support** — Create and manage multiple independent competitions, each with its own set of challenges
- **Challenge management** — Admin CRUD for challenges with static flag verification
- **In-competition flag submission** — Users submit flags within a competition context, with duplicate-solve prevention
- **Per-competition leaderboard** — Ranked by total score descending, ties broken by earliest solve time
- **Per-competition notifications** — Admin can post announcements scoped to a competition
- **Plugin system** — Extensible compile-time plugin interface for adding new features
- **Snowflake IDs** — All public-facing IDs use snowflake algorithm for distributed-friendly unique identifiers
- **JWT authentication** — Bearer token auth with admin role gating (user management handled externally)

## Quick Start

```bash
# Install dependencies
go mod download

# Configure
cp config.yaml.example config.yaml
# Edit config.yaml with your MySQL and JWT settings

# Apply database schema
mysql -u root -p your_db < sql/schema.sql

# Seed test data (optional)
go run ./cmd/seed/

# Run server
go run ./cmd/server -config config.yaml

# Try the demo script
./scripts/demo.sh
```

## API Overview

### Authentication

All endpoints require a Bearer JWT token in the `Authorization` header. Admin endpoints additionally require `role: admin` in the token claims.

### Challenges (Admin)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/admin/challenges` | Create challenge |
| PUT | `/api/v1/admin/challenges/{id}` | Update challenge |
| DELETE | `/api/v1/admin/challenges/{id}` | Delete challenge |

### Challenges (User)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/challenges` | List enabled challenges |
| GET | `/api/v1/challenges/{id}` | Get challenge detail |

### Submissions

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/challenges/{id}/submit` | Submit flag (global) |
| POST | `/api/v1/competitions/{comp_id}/challenges/{id}/submit` | Submit flag (in competition) |
| GET | `/api/v1/admin/submissions` | List submissions (admin) |

### Competitions (Admin)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/admin/competitions` | Create competition |
| PUT | `/api/v1/admin/competitions/{id}` | Update competition |
| DELETE | `/api/v1/admin/competitions/{id}` | Delete competition |
| GET | `/api/v1/admin/competitions` | List all competitions |
| POST | `/api/v1/admin/competitions/{id}/challenges` | Add challenge to competition |
| DELETE | `/api/v1/admin/competitions/{id}/challenges/{challenge_id}` | Remove challenge from competition |

### Competitions (User)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/competitions` | List active competitions |
| GET | `/api/v1/competitions/{id}` | Get competition detail |
| GET | `/api/v1/competitions/{id}/challenges` | List competition challenges |

### Plugins

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/competitions/{id}/leaderboard` | Competition leaderboard |
| POST | `/api/v1/admin/competitions/{id}/notifications` | Create competition notification |
| GET | `/api/v1/competitions/{id}/notifications` | List competition notifications |

## Tech Stack

- **Go 1.22** with [chi](https://github.com/go-chi/chi/v5) router
- **MySQL** via `database/sql`
- **JWT** (HS256) via `golang-jwt/jwt/v5`
- **Snowflake ID** — custom implementation for unique distributed IDs

## Project Structure

```
.
├── cmd/
│   ├── server/           # Entry point
│   └── seed/             # Test data seeder
├── scripts/
│   └── demo.sh           # Demo: query competitions, submit flags, leaderboard
├── internal/
│   ├── config/           # YAML config loading
│   ├── handler/          # HTTP handlers
│   ├── middleware/        # JWT auth, admin gate
│   ├── model/            # Domain structs
│   ├── plugin/           # Plugin interface
│   ├── service/          # Business logic
│   ├── snowflake/        # ID generator
│   ├── store/            # DB interfaces + MySQL impl
│   └── integration/      # Integration tests
├── plugins/
│   ├── leaderboard/      # Per-competition leaderboard
│   └── notification/     # Per-competition notifications
├── sql/schema.sql        # Database schema
└── config.yaml           # Configuration
```

## Configuration

```yaml
server:
  port: 8080

db:
  host: 127.0.0.1
  port: 3306
  user: root
  password: ""
  dbname: ctf

jwt:
  secret: "your-secret-key"
  admin_role: "admin"
```

## License

MIT
