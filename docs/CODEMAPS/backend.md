<!-- Generated: 2026-04-27 | Files scanned: 45 | Token estimate: ~750 -->

# Backend Architecture

## API Routes - CTF Server (8080)

### Competitions
```
GET    /api/v1/competitions                    → CompetitionHandler.List
POST   /api/v1/competitions                    → CompetitionHandler.Create (admin)
GET    /api/v1/competitions/:id                → CompetitionHandler.Get
PUT    /api/v1/competitions/:id                → CompetitionHandler.Update (admin)
DELETE /api/v1/competitions/:id                → CompetitionHandler.Delete (admin)
POST   /api/v1/competitions/:id/start          → CompetitionHandler.Start (admin)
POST   /api/v1/competitions/:id/end            → CompetitionHandler.End (admin)
GET    /api/v1/competitions/:id/challenges     → CompetitionHandler.ListChallenges
GET    /api/v1/competitions/:id/teams          → CompetitionHandler.ListTeams (admin, team mode)
POST   /api/v1/competitions/:id/teams          → CompetitionHandler.AddTeam (admin, team mode)
DELETE /api/v1/competitions/:id/teams/:teamID  → CompetitionHandler.RemoveTeam (admin, team mode)
```

### Challenges
```
GET    /api/v1/challenges       → ChallengeHandler.List
POST   /api/v1/challenges       → ChallengeHandler.Create (admin)
GET    /api/v1/challenges/:id   → ChallengeHandler.Get
PUT    /api/v1/challenges/:id   → ChallengeHandler.Update (admin)
DELETE /api/v1/challenges/:id   → ChallengeHandler.Delete (admin)
```

### Submissions
```
POST   /api/v1/submissions               → SubmissionHandler.Create
POST   /api/v1/competitions/:id/submit   → SubmissionHandler.SubmitInComp (rate limited)
```

### Plugin Routes
```
GET /api/v1/competitions/:id/leaderboard  → LeaderboardPlugin
GET /api/v1/topthree/competitions/:id     → TopThreePlugin
GET /api/v1/competitions/:id/analytics    → AnalyticsPlugin
GET /api/v1/competitions/:id/notifications → NotificationPlugin
GET /api/v1/competitions/:id/hints        → HintsPlugin
```

## API Routes - Auth Server (8081)

```
POST /api/v1/register          → AuthHandler.Register
POST /api/v1/login             → AuthHandler.Login
POST /api/v1/verify            → AuthHandler.Verify (called by CTF server middleware)

GET  /api/v1/teams             → TeamHandler.List
POST /api/v1/teams             → TeamHandler.Create
GET  /api/v1/teams/:id         → TeamHandler.Get
PUT  /api/v1/teams/:id         → TeamHandler.Update
DELETE /api/v1/teams/:id       → TeamHandler.Delete
POST /api/v1/teams/:id/members → TeamHandler.AddMember
DELETE /api/v1/teams/:id/members/:userID → TeamHandler.RemoveMember

PUT  /api/v1/admin/teams/:id/captain         → TeamHandler.SetCaptain (admin)
POST /api/v1/admin/teams/:id/transfer-captain → TeamHandler.TransferCaptain (admin)
```

## Middleware Chain
```
Request → Authenticate (auth service verify) → RateLimit → Handler
              ↓
         RequireAdmin (optional)
```

## Key Files
```
cmd/server/main.go           (bootstrap, 200 lines)
cmd/auth-server/main.go      (auth server bootstrap)
internal/router/             (route registration)
  ├── api.go
  ├── competitions.go
  ├── challenges.go
  └── submissions.go
internal/handler/            (HTTP handlers)
internal/service/            (business logic)
  ├── competition.go         (CheckCompAccess, team mode logic)
  ├── submission.go          (SubmitInComp, team deduplication)
  └── team_resolver.go       (GetUserTeam from auth service)
internal/store/              (data access, mysql.go)
internal/middleware/         (auth.go, ratelimit.go)
internal/event/              (EventCorrectSubmission with TeamID)
internal/plugin/             (plugin interface)
internal/pluginutil/         (shared queries for plugins)
plugins/topthree/            (topthree plugin)
plugins/leaderboard/         (leaderboard plugin, dual mode)
plugins/analytics/           (analytics plugin, dual mode)
```
