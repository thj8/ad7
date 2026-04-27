<!-- Generated: 2026-04-27 | Files scanned: 12 | Token estimate: ~700 -->

# Database Schema

## Core Tables
### competitions
```
id (INT, PK, AI)
res_id (VARCHAR(32), UNIQUE)
title (VARCHAR(255))
description (TEXT)
mode (VARCHAR(32))       -- 'individual'|'team'
team_join_mode (VARCHAR(32)) -- 'free'|'admin'
starts_at (DATETIME)
ends_at (DATETIME)
created_at, updated_at (DATETIME)
is_deleted (TINYINT)
```

### challenges
```
id (INT, PK, AI)
res_id (VARCHAR(32), UNIQUE)
title (VARCHAR(255))
description (TEXT)
category (VARCHAR(100))
score (INT)
flag (VARCHAR(255), JSON hidden)
created_at, updated_at (DATETIME)
is_deleted (TINYINT)
```

### competition_challenges
```
id (INT, PK, AI)
competition_id (INT)
challenge_id (INT)
created_at (DATETIME)
is_deleted (TINYINT)
```

### submissions
```
id (INT, PK, AI)
res_id (VARCHAR(32), UNIQUE)
competition_id (INT, NULLABLE)
challenge_id (INT)
user_id (VARCHAR(32))
team_id (VARCHAR(32), NULLABLE) -- team mode
flag (VARCHAR(255))
correct (TINYINT)
created_at (DATETIME)
is_deleted (TINYINT)
```

## Auth Tables
### users
```
id (INT, PK, AI)
res_id (VARCHAR(32), UNIQUE)
username (VARCHAR(255), UNIQUE)
password_hash (VARCHAR(255))
role (VARCHAR(32))
created_at, updated_at (DATETIME)
is_deleted (TINYINT)
```

### teams
```
id (INT, PK, AI)
res_id (VARCHAR(32), UNIQUE)
name (VARCHAR(255), UNIQUE)
created_at, updated_at (DATETIME)
is_deleted (TINYINT)
```

### team_members
```
id (INT, PK, AI)
team_id (INT)
user_id (INT)
role (VARCHAR(32)) -- 'captain'|'member'
created_at (DATETIME)
is_deleted (TINYINT)
UNIQUE KEY (team_id, user_id)
```

## Team Competition Table
### competition_teams
```
id (INT, PK, AI)
competition_id (INT)
team_id (INT)
created_at (DATETIME)
is_deleted (TINYINT)
UNIQUE KEY (competition_id, team_id)
```

## Plugin Tables
### topthree_records
```
id (INT, PK, AI)
res_id (VARCHAR(32), UNIQUE)
competition_id (INT)
challenge_id (INT)
user_id (VARCHAR(32))
team_id (VARCHAR(32), NULLABLE)
ranking (INT) -- 1|2|3
created_at (DATETIME)
is_deleted (TINYINT)
```

### notifications, hints, etc.
- Plugin-specific tables follow same pattern

## Migration History
```
sql/schema.sql              (initial schema)
sql/migrations/001_team_members.sql  (user-team relationship refactor)
sql/migrations/002_team_competition_mode.sql  (team competition mode)
```
