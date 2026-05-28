-- db/db_mysql/migrations/20260528000000_add_unique_index_video_progress.sql
-- +goose Up
ALTER TABLE `video_progresses` DROP INDEX `idx_video_progresses_user_result_video`;
ALTER TABLE `video_progresses` ADD UNIQUE INDEX `idx_video_progresses_unique_urv` (`user_id`, `result_id`, `video_id`);
-- +goose Down
ALTER TABLE `video_progresses` DROP INDEX `idx_video_progresses_unique_urv`;
ALTER TABLE `video_progresses` ADD INDEX `idx_video_progresses_user_result_video` (`user_id`, `result_id`, `video_id`);
