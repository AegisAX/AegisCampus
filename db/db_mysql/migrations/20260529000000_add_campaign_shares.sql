-- db/db_mysql/migrations/20260529000000_add_campaign_shares.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS `campaign_shares` (
    `id`           INT(11) NOT NULL AUTO_INCREMENT,
    `campaign_id`  INT(11) NOT NULL,
    `user_id`      INT(11) NOT NULL,
    `created_date` DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_campaign_shares_unique_cu` (`campaign_id`, `user_id`),
    KEY `idx_campaign_shares_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose Down
DROP TABLE IF EXISTS `campaign_shares`;