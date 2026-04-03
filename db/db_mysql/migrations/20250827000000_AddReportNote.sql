-- +goose Up
ALTER TABLE `results` ADD COLUMN report_note TEXT;

-- +goose Down
-- (옵션) SQLite는 컬럼 DROP이 번거로우므로 Down은 생략하거나 재생성 전략 사용

