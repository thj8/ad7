-- Competition mode and team join mode
ALTER TABLE competitions
    ADD COLUMN mode VARCHAR(16) NOT NULL DEFAULT 'individual',
    ADD COLUMN team_join_mode VARCHAR(16) NOT NULL DEFAULT 'free';

-- Competition teams association table (managed mode only)
CREATE TABLE IF NOT EXISTS competition_teams (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    res_id VARCHAR(32) NOT NULL UNIQUE,
    competition_id VARCHAR(32) NOT NULL,
    team_id VARCHAR(32) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    is_deleted TINYINT NOT NULL DEFAULT 0,
    UNIQUE INDEX idx_comp_team (competition_id, team_id, is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Submission team ID field
ALTER TABLE submissions
    ADD COLUMN team_id VARCHAR(32) DEFAULT NULL;

ALTER TABLE submissions
    ADD INDEX idx_team_chal_comp_correct (team_id, challenge_id, competition_id, is_correct);

-- Topthree records team ID field
ALTER TABLE topthree_records
    ADD COLUMN team_id VARCHAR(32) DEFAULT NULL;
