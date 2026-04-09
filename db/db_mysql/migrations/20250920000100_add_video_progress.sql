-- db/db_mysql/migrations/20250920000100_add_video_progress.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS `video_progresses` (
    `id`              BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id`         BIGINT,
    `result_id`       BIGINT,
    `video_id`        BIGINT NOT NULL,
    `seconds_watched` BIGINT DEFAULT 0,
    `duration`        BIGINT DEFAULT 0,
    `percent`         DOUBLE DEFAULT 0.0,
    `completed`       TINYINT(1) DEFAULT 0,
    `modified_date`   DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_video_progresses_user_result_video (user_id, result_id, video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS `video_progresses`;