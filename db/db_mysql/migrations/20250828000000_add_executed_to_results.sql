-- +goose Up
-- +goose StatementBegin
ALTER TABLE `results` ADD COLUMN `executed` TINYINT(1) NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE `results` DROP COLUMN `executed`;
-- +goose StatementEnd