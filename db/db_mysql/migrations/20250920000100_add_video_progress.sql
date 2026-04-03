-- +goose Up
-- 동영상 시청 진행률 추적 테이블 생성
CREATE TABLE IF NOT EXISTS `video_progress` (
    `id`                BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id`           BIGINT,
    `video_id`          BIGINT NOT NULL,
    `campaign_id`       BIGINT DEFAULT NULL,
    `email`             VARCHAR(255),
    `progress`          FLOAT DEFAULT 0,
    `complete`          BOOLEAN DEFAULT FALSE,
    `last_watched_date` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `created_date`      DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`video_id`) REFERENCES `videos`(`id`) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS `video_progress`;
