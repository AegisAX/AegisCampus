-- db/db_sqlite3/migrations/20260528000000_add_unique_index_video_progress.sql
-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_video_progresses_user_result_video;
CREATE UNIQUE INDEX IF NOT EXISTS idx_video_progresses_unique_urv ON video_progresses (user_id, result_id, video_id);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_video_progresses_unique_urv;
CREATE INDEX IF NOT EXISTS idx_video_progresses_user_result_video ON video_progresses (user_id, result_id, video_id);
-- +goose StatementEnd
