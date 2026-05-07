-- +goose Up
-- +goose StatementBegin
UPDATE `pages`
SET `html` = REPLACE(`html`,
    '" type="video/mp4"/>',
    '?rid={{.RId}}" type="video/mp4"/>')
WHERE `html` LIKE '%/media/%" type="video/mp4"/>%'
  AND `html` NOT LIKE '%?rid={{.RId}}" type="video/mp4"/>%';

UPDATE `pages`
SET `html` = REPLACE(`html`,
    '" type="video/mp4" />',
    '?rid={{.RId}}" type="video/mp4" />')
WHERE `html` LIKE '%/media/%" type="video/mp4" />%'
  AND `html` NOT LIKE '%?rid={{.RId}}" type="video/mp4" />%';

UPDATE `redirect_pages`
SET `html` = REPLACE(`html`,
    '" type="video/mp4"/>',
    '?rid={{.RId}}" type="video/mp4"/>')
WHERE `html` LIKE '%/media/%" type="video/mp4"/>%'
  AND `html` NOT LIKE '%?rid={{.RId}}" type="video/mp4"/>%';

UPDATE `redirect_pages`
SET `html` = REPLACE(`html`,
    '" type="video/mp4" />',
    '?rid={{.RId}}" type="video/mp4" />')
WHERE `html` LIKE '%/media/%" type="video/mp4" />%'
  AND `html` NOT LIKE '%?rid={{.RId}}" type="video/mp4" />%';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE `pages`
SET `html` = REPLACE(`html`,
    '?rid={{.RId}}" type="video/mp4"/>',
    '" type="video/mp4"/>')
WHERE `html` LIKE '%?rid={{.RId}}" type="video/mp4"/>%';

UPDATE `pages`
SET `html` = REPLACE(`html`,
    '?rid={{.RId}}" type="video/mp4" />',
    '" type="video/mp4" />')
WHERE `html` LIKE '%?rid={{.RId}}" type="video/mp4" />%';

UPDATE `redirect_pages`
SET `html` = REPLACE(`html`,
    '?rid={{.RId}}" type="video/mp4"/>',
    '" type="video/mp4"/>')
WHERE `html` LIKE '%?rid={{.RId}}" type="video/mp4"/>%';

UPDATE `redirect_pages`
SET `html` = REPLACE(`html`,
    '?rid={{.RId}}" type="video/mp4" />',
    '" type="video/mp4" />')
WHERE `html` LIKE '%?rid={{.RId}}" type="video/mp4" />%';
-- +goose StatementEnd