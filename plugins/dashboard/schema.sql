CREATE TABLE IF NOT EXISTS dashboard_first_blood (
    id INT AUTO_INCREMENT PRIMARY KEY,
    res_id BIGINT NOT NULL UNIQUE COMMENT '雪花ID',
    competition_id BIGINT NOT NULL COMMENT '比赛ID（0表示全局题）',
    challenge_id BIGINT NOT NULL COMMENT '题目ID',
    user_id VARCHAR(255) NOT NULL COMMENT '用户ID',
    created_at DATETIME NOT NULL COMMENT '一血时间',
    UNIQUE KEY idx_challenge_comp (challenge_id, competition_id),
    INDEX idx_competition (competition_id),
    INDEX idx_challenge (challenge_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='一血记录表';
