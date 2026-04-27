<!-- Generated: 2026-04-27 | Files scanned: 60 | Token estimate: ~650 -->

# CTF Platform Architecture

## Overview
Dual-service Go application:
- **Auth Server** (port 8081): User auth, team management
- **CTF Server** (port 8080): Competitions, challenges, submissions, plugins

## High-Level Flow
```
HTTP Request → Router → Middleware (Auth, RateLimit) → Handler → Service → Store → MySQL
                                    ↓
                                Event Bus → Plugins (Leaderboard, TopThree, Analytics, etc.)
```

## Service Boundaries
```
┌─────────────────────────────────────────────────────────────────┐
│                        CTF Server (8080)                        │
├─────────────────────────────────────────────────────────────────┤
│  Router  │  Handlers  │  Services  │  Store  │  MySQL Database  │
│          │            │            │         │                   │
│  + API   │  + Comp    │  + Comp    │  + CRUD │  - competitions  │
│  routes  │  + Chals   │  + Chals   │         │  - challenges    │
│          │  + Subs    │  + Subs    │         │  - submissions   │
│          │            │            │         │  - plugin tables │
└─────────────────────────────────────────────────────────────────┘
                            ↑ HTTP
┌─────────────────────────────────────────────────────────────────┐
│                      Auth Server (8081)                         │
├─────────────────────────────────────────────────────────────────┤
│  /register  /login  /verify  /teams/*  /admin/teams/*            │
│                                                                   │
│  - users table          - teams table         - team_members     │
└─────────────────────────────────────────────────────────────────┘
```

## Key Design Decisions
- Two services share MySQL database, communicate via HTTP
- Plugin system for extendable features (leaderboard, topthree, etc.)
- Event-driven plugin updates (EventCorrectSubmission)
- Soft-deletes only (is_deleted flag)
- Public IDs: res_id (UUID v4, 32-char hex), internal: auto-increment id
- No foreign keys for flexibility
