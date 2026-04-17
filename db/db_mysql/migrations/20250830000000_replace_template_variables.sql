-- +goose Up
-- +goose StatementBegin
UPDATE `templates`
SET `subject` = REPLACE(REPLACE(`subject`,
    '{{.FirstName}}', '{{.Name}}'),
    '{{.LastName}}',  '{{.Department}}')
WHERE `subject` LIKE '%{{.FirstName}}%'
   OR `subject` LIKE '%{{.LastName}}%';

UPDATE `templates`
SET `html` = REPLACE(REPLACE(`html`,
    '{{.FirstName}}', '{{.Name}}'),
    '{{.LastName}}',  '{{.Department}}')
WHERE `html` LIKE '%{{.FirstName}}%'
   OR `html` LIKE '%{{.LastName}}%';

UPDATE `templates`
SET `text` = REPLACE(REPLACE(`text`,
    '{{.FirstName}}', '{{.Name}}'),
    '{{.LastName}}',  '{{.Department}}')
WHERE `text` LIKE '%{{.FirstName}}%'
   OR `text` LIKE '%{{.LastName}}%';

UPDATE `pages`
SET `html` = REPLACE(REPLACE(`html`,
    '{{.FirstName}}', '{{.Name}}'),
    '{{.LastName}}',  '{{.Department}}')
WHERE `html` LIKE '%{{.FirstName}}%'
   OR `html` LIKE '%{{.LastName}}%';

UPDATE `pages`
SET `redirect_url` = REPLACE(REPLACE(`redirect_url`,
    '{{.FirstName}}', '{{.Name}}'),
    '{{.LastName}}',  '{{.Department}}')
WHERE `redirect_url` LIKE '%{{.FirstName}}%'
   OR `redirect_url` LIKE '%{{.LastName}}%';

UPDATE `smtp_headers`
SET `key` = REPLACE(REPLACE(`key`,
    '{{.FirstName}}', '{{.Name}}'),
    '{{.LastName}}',  '{{.Department}}')
WHERE `key` LIKE '%{{.FirstName}}%'
   OR `key` LIKE '%{{.LastName}}%';

UPDATE `smtp_headers`
SET `value` = REPLACE(REPLACE(`value`,
    '{{.FirstName}}', '{{.Name}}'),
    '{{.LastName}}',  '{{.Department}}')
WHERE `value` LIKE '%{{.FirstName}}%'
   OR `value` LIKE '%{{.LastName}}%';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE `templates`
SET `subject` = REPLACE(REPLACE(`subject`,
    '{{.Name}}', '{{.FirstName}}'),
    '{{.Department}}', '{{.LastName}}')
WHERE `subject` LIKE '%{{.Name}}%'
   OR `subject` LIKE '%{{.Department}}%';

UPDATE `templates`
SET `html` = REPLACE(REPLACE(`html`,
    '{{.Name}}', '{{.FirstName}}'),
    '{{.Department}}', '{{.LastName}}')
WHERE `html` LIKE '%{{.Name}}%'
   OR `html` LIKE '%{{.Department}}%';

UPDATE `templates`
SET `text` = REPLACE(REPLACE(`text`,
    '{{.Name}}', '{{.FirstName}}'),
    '{{.Department}}', '{{.LastName}}')
WHERE `text` LIKE '%{{.Name}}%'
   OR `text` LIKE '%{{.Department}}%';

UPDATE `pages`
SET `html` = REPLACE(REPLACE(`html`,
    '{{.Name}}', '{{.FirstName}}'),
    '{{.Department}}', '{{.LastName}}')
WHERE `html` LIKE '%{{.Name}}%'
   OR `html` LIKE '%{{.Department}}%';

UPDATE `pages`
SET `redirect_url` = REPLACE(REPLACE(`redirect_url`,
    '{{.Name}}', '{{.FirstName}}'),
    '{{.Department}}', '{{.LastName}}')
WHERE `redirect_url` LIKE '%{{.Name}}%'
   OR `redirect_url` LIKE '%{{.Department}}%';

UPDATE `smtp_headers`
SET `key` = REPLACE(REPLACE(`key`,
    '{{.Name}}', '{{.FirstName}}'),
    '{{.Department}}', '{{.LastName}}')
WHERE `key` LIKE '%{{.Name}}%'
   OR `key` LIKE '%{{.Department}}%';

UPDATE `smtp_headers`
SET `value` = REPLACE(REPLACE(`value`,
    '{{.Name}}', '{{.FirstName}}'),
    '{{.Department}}', '{{.LastName}}')
WHERE `value` LIKE '%{{.Name}}%'
   OR `value` LIKE '%{{.Department}}%';
-- +goose StatementEnd