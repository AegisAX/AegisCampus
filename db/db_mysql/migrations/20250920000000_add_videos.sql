-- +goose Up
-- 동영상 관리 테이블 생성 (보안 교육 동영상)
-- 20250920000000_add_videos.sql (MySQL 수정본)
CREATE TABLE IF NOT EXISTS `videos` (
    `id`               BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id`          BIGINT,
    `name`             VARCHAR(255) NOT NULL,
    `description`      TEXT,
    `file_name`        VARCHAR(255) NOT NULL,
    `file_path`        VARCHAR(500) NOT NULL,
    `thumbnail_path`   VARCHAR(500),
    `duration_seconds` BIGINT DEFAULT 0,
    `is_public`        TINYINT(1) DEFAULT 0,
    `created_date`     DATETIME DEFAULT CURRENT_TIMESTAMP,
    `modified_date`    DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_videos_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose Down
DROP TABLE IF EXISTS `videos`;