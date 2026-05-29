-- db/db_sqlite3/migrations/20260529000000_add_campaign_shares.sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS campaign_shares (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id  INTEGER NOT NULL,
    user_id      INTEGER NOT NULL,
    created_date DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_campaign_shares_unique_cu ON campaign_shares (campaign_id, user_id);
CREATE INDEX IF NOT EXISTS idx_campaign_shares_user_id ON campaign_shares (user_id);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_campaign_shares_user_id;
DROP INDEX IF EXISTS idx_campaign_shares_unique_cu;
DROP TABLE IF EXISTS campaign_shares;
-- +goose StatementEnd