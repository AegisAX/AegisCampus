-- db/db_sqlite3/migrations/20250920000100_add_video_progress.sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS video_progresses (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER,
    result_id       INTEGER,
    video_id        INTEGER  NOT NULL,
    seconds_watched INTEGER  DEFAULT 0,
    duration        INTEGER  DEFAULT 0,
    percent         REAL     DEFAULT 0.0,
    completed       INTEGER  DEFAULT 0,
    modified_date   DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_video_progresses_user_result_video
ON video_progresses (user_id, result_id, video_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_video_progresses_user_result_video;
DROP TABLE IF EXISTS video_progresses;
-- +goose StatementEnd