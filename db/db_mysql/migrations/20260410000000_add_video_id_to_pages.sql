-- +goose Up
-- +goose StatementBegin
ALTER TABLE pages ADD COLUMN video_id INTEGER DEFAULT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite does not support DROP COLUMN; migration is irreversible
-- +goose StatementEnd