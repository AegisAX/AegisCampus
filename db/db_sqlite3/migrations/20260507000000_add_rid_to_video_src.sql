-- +goose Up
-- +goose StatementBegin
-- 영상 src 에 ?rid={{.RId}} 토큰 추가
-- (Phase 2 #7 — /media/{id} 의 rid 검증을 위한 데이터 마이그레이션)
--
-- 두 가지 self-closing 패턴 모두 처리:
--   (A) <source src="/media/N" type="video/mp4"/>   (slash 직전 공백 없음)
--   (B) <source src="/media/N" type="video/mp4" />  (slash 직전 공백 1개)
-- 각 테이블(pages, redirect_pages)에 두 패턴씩 총 4개 UPDATE.
-- 멱등성: 이미 ?rid={{.RId}} 가 있는 행은 WHERE 로 제외.

-- pages — 패턴 A (공백 없는 self-closing)
UPDATE pages
SET html = REPLACE(html,
    '" type="video/mp4"/>',
    '?rid={{.RId}}" type="video/mp4"/>')
WHERE html LIKE '%/media/%" type="video/mp4"/>%'
  AND html NOT LIKE '%?rid={{.RId}}" type="video/mp4"/>%';

-- pages — 패턴 B (공백 있는 self-closing)
UPDATE pages
SET html = REPLACE(html,
    '" type="video/mp4" />',
    '?rid={{.RId}}" type="video/mp4" />')
WHERE html LIKE '%/media/%" type="video/mp4" />%'
  AND html NOT LIKE '%?rid={{.RId}}" type="video/mp4" />%';

-- redirect_pages — 패턴 A
UPDATE redirect_pages
SET html = REPLACE(html,
    '" type="video/mp4"/>',
    '?rid={{.RId}}" type="video/mp4"/>')
WHERE html LIKE '%/media/%" type="video/mp4"/>%'
  AND html NOT LIKE '%?rid={{.RId}}" type="video/mp4"/>%';

-- redirect_pages — 패턴 B
UPDATE redirect_pages
SET html = REPLACE(html,
    '" type="video/mp4" />',
    '?rid={{.RId}}" type="video/mp4" />')
WHERE html LIKE '%/media/%" type="video/mp4" />%'
  AND html NOT LIKE '%?rid={{.RId}}" type="video/mp4" />%';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Down: ?rid={{.RId}} 토큰 제거 (역방향, 두 패턴)

UPDATE pages
SET html = REPLACE(html,
    '?rid={{.RId}}" type="video/mp4"/>',
    '" type="video/mp4"/>')
WHERE html LIKE '%?rid={{.RId}}" type="video/mp4"/>%';

UPDATE pages
SET html = REPLACE(html,
    '?rid={{.RId}}" type="video/mp4" />',
    '" type="video/mp4" />')
WHERE html LIKE '%?rid={{.RId}}" type="video/mp4" />%';

UPDATE redirect_pages
SET html = REPLACE(html,
    '?rid={{.RId}}" type="video/mp4"/>',
    '" type="video/mp4"/>')
WHERE html LIKE '%?rid={{.RId}}" type="video/mp4"/>%';

UPDATE redirect_pages
SET html = REPLACE(html,
    '?rid={{.RId}}" type="video/mp4" />',
    '" type="video/mp4" />')
WHERE html LIKE '%?rid={{.RId}}" type="video/mp4" />%';

-- +goose StatementEnd