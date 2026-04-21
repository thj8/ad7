-- User-Team Relationship Redesign Migration
-- Date: 2026-04-21
-- Creates team_members table and migrates existing users.team_id data

-- 1. Create team_members table
CREATE TABLE IF NOT EXISTS team_members (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    res_id     VARCHAR(32)   NOT NULL UNIQUE,
    team_id    VARCHAR(32)   NOT NULL,
    user_id    VARCHAR(32)   NOT NULL,
    role       VARCHAR(64)   NOT NULL DEFAULT 'member',
    is_deleted TINYINT(1)    NOT NULL DEFAULT 0,
    created_at DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_team (team_id),
    INDEX idx_user (user_id),
    UNIQUE INDEX idx_team_user_active (team_id, user_id, is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='队伍成员关系表';

-- 2. Migrate existing users.team_id -> team_members
-- The earliest registered user (lowest users.id) per team becomes captain
INSERT INTO team_members (res_id, team_id, user_id, role, is_deleted, created_at, updated_at)
SELECT
    REPLACE(UUID(), '-', '') AS res_id,
    u.team_id,
    u.res_id AS user_id,
    CASE WHEN u.id = (
        SELECT MIN(u2.id)
        FROM users u2
        WHERE u2.team_id = u.team_id
          AND u2.is_deleted = 0
    ) THEN 'captain' ELSE 'member' END AS role,
    0 AS is_deleted,
    u.created_at,
    u.updated_at
FROM users u
WHERE u.team_id IS NOT NULL
  AND u.team_id != ''
  AND u.is_deleted = 0;

-- 3. Drop old column from users table
-- First drop index first, then column
ALTER TABLE users DROP INDEX IF EXISTS idx_team;
ALTER TABLE users DROP COLUMN IF EXISTS team_id;
