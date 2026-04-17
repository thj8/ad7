CREATE DATABASE IF NOT EXISTS ctf CHARACTER SET utf8mb4;
USE ctf;

CREATE TABLE IF NOT EXISTS challenges (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    res_id      BIGINT       NOT NULL UNIQUE,
    title       VARCHAR(255) NOT NULL,
    category    VARCHAR(64)  NOT NULL DEFAULT 'misc',
    description TEXT         NOT NULL,
    score       INT          NOT NULL DEFAULT 100,
    flag        VARCHAR(255) NOT NULL,
    is_enabled  TINYINT(1)   NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS submissions (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         BIGINT       NOT NULL UNIQUE,
    user_id        VARCHAR(128) NOT NULL,
    challenge_id   BIGINT       NOT NULL,
    competition_id BIGINT       NULL,
    submitted_flag VARCHAR(255) NOT NULL,
    is_correct     TINYINT(1)   NOT NULL,
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_challenge (user_id, challenge_id)
);

CREATE TABLE IF NOT EXISTS notifications (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         BIGINT       NOT NULL UNIQUE,
    competition_id BIGINT       NULL,
    challenge_id   BIGINT       NULL,
    title          VARCHAR(255) NOT NULL,
    message        TEXT         NOT NULL,
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS competitions (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    res_id      BIGINT       NOT NULL UNIQUE,
    title       VARCHAR(255) NOT NULL,
    description VARCHAR(4096) NOT NULL DEFAULT '',
    start_time  DATETIME     NOT NULL,
    end_time    DATETIME     NOT NULL,
    is_active   TINYINT(1)   NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS competition_challenges (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    res_id         BIGINT NOT NULL UNIQUE,
    competition_id BIGINT NOT NULL,
    challenge_id   BIGINT NOT NULL,
    UNIQUE INDEX idx_comp_chal (competition_id, challenge_id)
);
