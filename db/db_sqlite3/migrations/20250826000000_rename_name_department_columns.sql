-- +goose Up
-- +goose StatementBegin

-- targets: first_name → name, last_name → department
ALTER TABLE targets RENAME COLUMN first_name TO name;
ALTER TABLE targets RENAME COLUMN last_name TO department;

-- results: first_name → name, last_name → department
ALTER TABLE results RENAME COLUMN first_name TO name;
ALTER TABLE results RENAME COLUMN last_name TO department;

-- email_requests: first_name → name, last_name → department
ALTER TABLE email_requests RENAME COLUMN first_name TO name;
ALTER TABLE email_requests RENAME COLUMN last_name TO department;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE targets RENAME COLUMN name TO first_name;
ALTER TABLE targets RENAME COLUMN department TO last_name;
ALTER TABLE results RENAME COLUMN name TO first_name;
ALTER TABLE results RENAME COLUMN department TO last_name;
ALTER TABLE email_requests RENAME COLUMN name TO first_name;
ALTER TABLE email_requests RENAME COLUMN department TO last_name;
-- +goose StatementEnd