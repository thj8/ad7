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
