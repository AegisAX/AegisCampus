-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS redirect_pages (
    id            BIGINT        NOT NULL AUTO_INCREMENT,
    user_id       BIGINT        NOT NULL,
    name          VARCHAR(255)  NOT NULL,
    html          LONGTEXT,
    video_id      BIGINT        DEFAULT NULL,
    redirect_url  VARCHAR(2048) DEFAULT '',
    modified_date DATETIME      DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_redirect_pages_user_id (user_id),
    INDEX idx_redirect_pages_video_id (video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS redirect_pages;
-- +goose StatementEnd
