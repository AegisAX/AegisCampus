-- +goose Up
CREATE TABLE IF NOT EXISTS `user_preferences` (
    `user_id`                   INT(11) NOT NULL,
    `dashboard_campaign_filter` TEXT,
    `modified_date`             DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose Down
DROP TABLE IF EXISTS `user_preferences`;