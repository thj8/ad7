# Hints Plugin Design

**Date:** 2026-04-17
**Author:** Claude Code
**Status:** Draft

## Overview

Add a hints (提示) plugin to the AD7 CTF platform that allows admins to add hints to challenges and users to view them.

## Goals

- Allow admins to create, edit, and delete hints for specific challenges
- Allow users to list visible hints for a challenge
- Keep it simple - free, no unlock conditions, no ordering requirements
- Follow the existing plugin pattern (like notification and analytics)

## Non-Goals

- No score cost for viewing hints
- No time-based unlocks
- No sequential unlock requirements
- No Markdown/HTML support (pure text only)

## Data Model

### New Table: `hints`

```sql
CREATE TABLE IF NOT EXISTS hints (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    res_id      BIGINT       NOT NULL UNIQUE,
    challenge_id BIGINT      NOT NULL,
    content     TEXT         NOT NULL,
    is_visible  TINYINT(1)   NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Notes:**
- No foreign keys (per project constraint)
- `challenge_id` references `challenges.res_id` (not internal id)
- `is_visible` allows admins to hide hints temporarily without deleting them
- Order by `created_at ASC` when displaying to users

## API Endpoints

### Admin Endpoints (Require Admin Role)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/admin/challenges/{id}/hints` | Add a hint to a challenge |
| PUT | `/api/v1/admin/hints/{id}` | Update hint content or visibility |
| DELETE | `/api/v1/admin/hints/{id}` | Delete a hint |

### User Endpoints (Require Authentication)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/challenges/{id}/hints` | List visible hints for a challenge |

## Request/Response Formats

### Create Hint (POST /api/v1/admin/challenges/{id}/hints)

**Request:**
```json
{
    "content": "Try looking at the JavaScript console"
}
```

**Response:** 201 Created

### Update Hint (PUT /api/v1/admin/hints/{id})

**Request:**
```json
{
    "content": "Updated hint content",
    "is_visible": true
}
```

**Response:** 204 No Content

### List Hints (GET /api/v1/challenges/{id}/hints)

**Response:**
```json
{
    "hints": [
        {
            "id": 1234567890123456789,
            "content": "Try looking at the JavaScript console",
            "created_at": "2026-04-17T10:30:00Z"
        }
    ]
}
```

**Note:** Only returns hints where `is_visible = 1`

## Implementation Structure

### File Location

```
plugins/hints/hints.go
```

### Plugin Structure

Follow the same pattern as `notification` and `analytics` plugins:

```go
package hints

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"

    "ad7/internal/middleware"
    "ad7/internal/snowflake"
)

type Plugin struct{ db *sql.DB }

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
    p.db = db
    // Register routes here
}
```

## Integration Tests

Add tests to `internal/integration/` covering:

1. Admin can create a hint for a challenge
2. Admin can update a hint
3. Admin can delete a hint
4. User can list visible hints (invisible hints are not shown)
5. Hints are ordered by created_at ascending

## Registration in main.go

Add to the plugins slice:

```go
plugins := []plugin.Plugin{
    leaderboard.New(),
    notification.New(),
    analytics.New(),
    hints.New(), // <-- add this
}
```

## Error Handling

Follow existing patterns:
- Invalid IDs: 400 Bad Request
- Not found: 404 Not Found
- DB errors: 500 Internal Server Error
- Input validation: content is required (max 4096 chars like description)
