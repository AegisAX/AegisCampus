-- +goose Up
-- 동영상 관리 테이블 생성 (보안 교육 동영상)
CREATE TABLE IF NOT EXISTS `videos` (
    `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id`       BIGINT,
    `title`         VARCHAR(255) NOT NULL,
    `description`   TEXT,
    `url`           VARCHAR(500) NOT NULL,
    `created_date`  DATETIME DEFAULT CURRENT_TIMESTAMP,
    `modified_date` DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS `videos`;
