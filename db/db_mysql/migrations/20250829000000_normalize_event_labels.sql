-- +goose Up
-- +goose StatementBegin
UPDATE `events` SET `message` = 'Sent'      WHERE `message` = 'Email Sent';
UPDATE `events` SET `message` = 'Opened'    WHERE `message` = 'Email Opened';
UPDATE `events` SET `message` = 'Clicked'   WHERE `message` = 'Clicked Link';
UPDATE `events` SET `message` = 'Submitted' WHERE `message` = 'Submitted Data';
UPDATE `events` SET `message` = 'Reported'  WHERE `message` = 'Email Reported';

UPDATE `results` SET `status` = 'Sent'      WHERE `status` = 'Email Sent';
UPDATE `results` SET `status` = 'Opened'    WHERE `status` = 'Email Opened';
UPDATE `results` SET `status` = 'Clicked'   WHERE `status` = 'Clicked Link';
UPDATE `results` SET `status` = 'Submitted' WHERE `status` = 'Submitted Data';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE `events` SET `message` = 'Email Sent'      WHERE `message` = 'Sent';
UPDATE `events` SET `message` = 'Email Opened'    WHERE `message` = 'Opened';
UPDATE `events` SET `message` = 'Clicked Link'    WHERE `message` = 'Clicked';
UPDATE `events` SET `message` = 'Submitted Data'  WHERE `message` = 'Submitted';
UPDATE `events` SET `message` = 'Email Reported'  WHERE `message` = 'Reported';

UPDATE `results` SET `status` = 'Email Sent'      WHERE `status` = 'Sent';
UPDATE `results` SET `status` = 'Email Opened'    WHERE `status` = 'Opened';
UPDATE `results` SET `status` = 'Clicked Link'    WHERE `status` = 'Clicked';
UPDATE `results` SET `status` = 'Submitted Data'  WHERE `status` = 'Submitted';
-- +goose StatementEnd