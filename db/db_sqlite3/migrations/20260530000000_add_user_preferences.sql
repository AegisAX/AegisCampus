-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_preferences (
    user_id                   INTEGER PRIMARY KEY,
    dashboard_campaign_filter TEXT DEFAULT '',
    modified_date             DATETIME DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_preferences;
-- +goose StatementEnd