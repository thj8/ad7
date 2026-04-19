CREATE DATABASE IF NOT EXISTS ctf CHARACTER SET utf8mb4;
USE ctf;

CREATE TABLE IF NOT EXISTS challenges (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    res_id      VARCHAR(32)  NOT NULL UNIQUE,
    title       VARCHAR(255) NOT NULL,
    category    VARCHAR(64)  NOT NULL DEFAULT 'misc',
    description TEXT         NOT NULL,
    score       INT          NOT NULL DEFAULT 100,
    flag        VARCHAR(255) NOT NULL,
    is_enabled  TINYINT(1)   NOT NULL DEFAULT 1,
    is_deleted  TINYINT(1)   NOT NULL DEFAULT 0,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS submissions (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32)  NOT NULL UNIQUE,
    user_id        VARCHAR(128) NOT NULL,
    challenge_id   VARCHAR(32)  NOT NULL,
    competition_id VARCHAR(32)  NOT NULL,
    submitted_flag VARCHAR(255) NOT NULL,
    is_correct     TINYINT(1)   NOT NULL,
    is_deleted     TINYINT(1)   NOT NULL DEFAULT 0,
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_challenge (user_id, challenge_id),
    UNIQUE INDEX idx_user_chal_comp_correct (user_id, challenge_id, competition_id, is_correct)
);

CREATE TABLE IF NOT EXISTS notifications (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32)  NOT NULL UNIQUE,
    competition_id VARCHAR(32)  NOT NULL,
    title          VARCHAR(255) NOT NULL,
    message        TEXT         NOT NULL,
    is_deleted     TINYINT(1)   NOT NULL DEFAULT 0,
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS competitions (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    res_id      VARCHAR(32)  NOT NULL UNIQUE,
    title       VARCHAR(255) NOT NULL,
    description VARCHAR(4096) NOT NULL DEFAULT '',
    start_time  DATETIME     NOT NULL,
    end_time    DATETIME     NOT NULL,
    is_active   TINYINT(1)   NOT NULL DEFAULT 1,
    is_deleted  TINYINT(1)   NOT NULL DEFAULT 0,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS competition_challenges (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32) NOT NULL UNIQUE,
    competition_id VARCHAR(32) NOT NULL,
    challenge_id   VARCHAR(32) NOT NULL,
    is_deleted     TINYINT(1)  NOT NULL DEFAULT 0,
    created_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_comp_chal (competition_id, challenge_id)
);

CREATE TABLE IF NOT EXISTS hints (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    res_id      VARCHAR(32)  NOT NULL UNIQUE,
    challenge_id VARCHAR(32) NOT NULL,
    content     TEXT         NOT NULL,
    is_visible  TINYINT(1)   NOT NULL DEFAULT 1,
    is_deleted  TINYINT(1)   NOT NULL DEFAULT 0,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS topthree_records (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         VARCHAR(32)  NOT NULL UNIQUE COMMENT 'UUID',
    competition_id VARCHAR(32)  NOT NULL COMMENT '比赛ID',
    challenge_id   VARCHAR(32)  NOT NULL COMMENT '题目ID',
    user_id        VARCHAR(128) NOT NULL COMMENT '用户ID',
    ranking        TINYINT      NOT NULL COMMENT '排名 1-3',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '解题时间',
    updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    is_deleted     TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '软删除标记',
    UNIQUE INDEX idx_comp_chal_rank (competition_id, challenge_id, ranking),
    INDEX idx_comp_chal (competition_id, challenge_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每道题前三名记录表';
