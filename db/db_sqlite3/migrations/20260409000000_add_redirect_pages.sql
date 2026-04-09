-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS redirect_pages (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER  NOT NULL,
    name          TEXT     NOT NULL,
    html          TEXT     DEFAULT '',
    video_id      INTEGER  DEFAULT NULL,
    redirect_url  TEXT     DEFAULT '',
    modified_date DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_redirect_pages_user_id ON redirect_pages (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_redirect_pages_user_id;
DROP TABLE IF EXISTS redirect_pages;
-- +goose StatementEnd
