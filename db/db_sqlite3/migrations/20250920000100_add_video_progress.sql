-- +migrate Up
CREATE TABLE IF NOT EXISTS video_progresses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    result_id INTEGER,
    video_id INTEGER,
    seconds_watched INTEGER DEFAULT 0,
    duration INTEGER DEFAULT 0,
    percent REAL DEFAULT 0.0,
    completed BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Add an index to speed lookups by (user_id, result_id, video_id)
CREATE INDEX IF NOT EXISTS idx_video_progresses_user_result_video
ON video_progresses (user_id, result_id, video_id);

-- +migrate Down
DROP INDEX IF EXISTS idx_video_progresses_user_result_video;
DROP TABLE IF EXISTS video_progresses;

