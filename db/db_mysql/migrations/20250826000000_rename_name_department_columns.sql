-- +goose Up
-- +goose StatementBegin
ALTER TABLE targets
    CHANGE COLUMN first_name name varchar(255),
    CHANGE COLUMN last_name department varchar(255);

ALTER TABLE results
    CHANGE COLUMN first_name name varchar(255),
    CHANGE COLUMN last_name department varchar(255);

ALTER TABLE email_requests
    CHANGE COLUMN first_name name varchar(255),
    CHANGE COLUMN last_name department varchar(255);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE targets
    CHANGE COLUMN name first_name varchar(255),
    CHANGE COLUMN department last_name varchar(255);

ALTER TABLE results
    CHANGE COLUMN name first_name varchar(255),
    CHANGE COLUMN department last_name varchar(255);

ALTER TABLE email_requests
    CHANGE COLUMN name first_name varchar(255),
    CHANGE COLUMN department last_name varchar(255);
-- +goose StatementEnd