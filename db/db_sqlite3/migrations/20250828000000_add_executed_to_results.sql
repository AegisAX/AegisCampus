-- +goose Up
-- +goose StatementBegin
ALTER TABLE results ADD COLUMN executed BOOLEAN DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite 3.35.0 이상에서만 DROP COLUMN 지원
-- 하위 호환이 필요하면 이 Down을 실행하지 않고 수동으로 재생성해야 합니다.
ALTER TABLE results DROP COLUMN executed;
-- +goose StatementEnd