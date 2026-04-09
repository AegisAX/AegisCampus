-- +goose Up
ALTER TABLE results ADD COLUMN report_note TEXT;

-- +goose Down
ALTER TABLE results DROP COLUMN report_note;
