# Changelog

이 프로젝트의 모든 변경 사항은 이 파일에 기록됩니다.

[Keep a Changelog](https://keepachangelog.com/ko/1.1.0/) 형식을 따르며,
이 프로젝트는 [Semantic Versioning](https://semver.org/spec/v2.0.0.html) 을 사용합니다.

## [Unreleased]

### Fixed (운영 검증 — rc2 후 발견)

- **#43** 캠페인 상세 페이지 (`/campaigns/{id}`) 초기 로드 성능 결함 보강.
  `static/js/src/app/campaign_results.js` 의 `load()` 가
  `indexEventTimesByEmail` (timeline 전체 순회) 을 results 루프 안에서 매
  iteration 호출하던 O(N×M) 구조 차단. 루프 밖으로 hoist. 824 results ×
  1661 events 환경에서 페이지 로드 ~20s → ~1s. `poll()` 의 동일 패턴과
  일관성 확보.
- **#48** 캠페인 상세 페이지 Timeline 차트 X축 범위가 첫 로드와 Refresh
  사이에서 달라지던 결함 보강. `load()` 는 `Campaign Created` 이벤트를
  series 에서 제외해 X축이 실제 발송 구간으로 자동 잡혔으나, `poll()` 은
  제외하지 않아 Refresh 후 X축이 캠페인 생성 시각까지 펼쳐졌다. `poll()`
  도 `load()` 와 동일하게 `Campaign Created` skip 추가.

### Phase 5+ — 출시 후 (v1.1 영역)
- TestAttachment 사전 결함 (testdata 옛 변수)
- `/videos/upload` 제거 + JS 를 `/api/videos/` 로 이전
- Email Template / Landing Page / Redirect Page 권한 체계 통일

---

## [1.0.0-rc2] - 2026-05-27

v1.0.0-rc1 운영 검증 중 발견된 보안 결함 1건 + 결함 4건 + UX 개선 1건을
수정한 RC 패치 릴리스. 신규 기능 추가 없음, 마이그레이션 추가 없음.

### Security (보안 결함 차단)

- **#41** Media·RedirectPage 핸들러의 cross-campaign 자산 접근 차단.
  기존엔 rid 의 `result.UserId` 만 검증해, 같은 운영자가 보유한 다른
  캠페인의 영상/RP 자산이 cross-access 가능하던 결함이 있었다.
  이제 LandingPage.RedirectUrl 의 `/rp/{id}` 패턴으로 캠페인↔RP 매핑을
  유도해 `rid → Result → Campaign → LP 또는 RP 의 자산 ID 일치`
  여부까지 검증. `ExtractRedirectPageID` 헬퍼 신규, RP 핸들러는 rid
  필수화. (rc1 #38 의 후속 강화)

### Fixed (안정성 / 동작 결함)

- **#42** Dashboard pie chart 의 Trained 카운트가 1로만 표시되던 결함.
  `getCampaignStats` 의 SubQuery + `email IN (?)` 패턴이 GORM v1 +
  SQLite 조합에서 1을 반환하던 결함을 `COUNT(DISTINCT email)` raw SQL
  로 교체. Campaigns 페이지의 정상 카운트와 일치.
- **#44** 캠페인 상세 페이지 Refresh 시 Details 행의 시간 컬럼
  (Sent/Opened/Clicked/Submitted/Executed/Reported/Trained at) 이
  새 이벤트로 갱신되지 않던 결함. `poll()` 의 update 루프 진입 전
  `indexEventTimesByEmail` 1회 인덱싱 후 행마다 시간 컬럼 갱신.
- **#45** 도넛 차트 가운데 숫자가 좌측 정렬되거나 리사이즈 후 stale
  좌표로 그려지던 결함. `load` 콜백에 `dominant-baseline: central`,
  `render` 콜백에 `plotLeft/plotTop` 기반 좌표 재계산 추가.
- **#46** 도넛 차트 7개 중 마지막 하나(Trained)만 둘째 줄 첫 column
  으로 튀던 결함. Bootstrap 3 `.row::before` 의 clearfix `display:
  table` 이 grid container 의 첫 cell 을 점유하던 것이 root cause.
  `.donut-row` 클래스 신규로 `display:grid; repeat(7, minmax(0, 1fr))`
  + `::before/::after` 무력화. viewport 640px / 400px 임계점에서
  4열 / 3열로 재배치.

### Changed (UX 개선)

- **#47** 도넛 차트 가운데 숫자 + 위 라벨 (Sent/Opened/...) 폰트가
  cell width 무관하게 16px 로 고정되어 좁은 grid 에서 도넛과 겹치던
  UX 결함 개선. `pickFontSizes(chartWidth)` 헬퍼로 ≥160px → 16px,
  ≥120px → 14px, ≥90px → 12px, 그 외 10px 의 단계별 폰트 적용.
  `main.css` 의 `.highcharts-title` 가 `!important` 로 인라인 override
  를 차단하던 것도 제거.

---

## [1.0.0-rc1] - 2026-05-19

v1.0.0-beta2 운영 검증 후, 출시 후보(rc) 승격을 위한 기능 안정화 릴리스.
신규 기능 추가 없음. 운영 중 발견된 결함 + rc1 직전 전수 코드 감사에서
발견된 결함을 모두 차단하고, 회귀 방지 테스트를 추가했다.

### Security (보안 / 정합성 결함 차단)

- **#2** admin `/videos/stream/{id}`·`/media/{id}` 의 cross-tenant IDOR 차단.
  소유자 아님 + is_public 아님이면 404. 영상 목록 쿼리의
  `user_id = ? OR is_public` 불변식과 일치시켜 일관성 확보.
- **F1** `/track/video` 서버 권위화. 기존엔 클라이언트가 보낸 `completed`
  불리언과 무제한 `seconds_watched` 를 그대로 신뢰해, rc1 #3 의 서버 권위
  완료판정(`TrainingCompleteHandler`)을 우회하는 백도어가 존재했다.
  이제 클라 `completed` 는 무시하고, `seconds_watched` 는 서버 보유
  `videos.duration_seconds` 로 상한 클램프하며, percent/완료는 서버 길이
  기준으로만 계산한다. (#10/#11 정상 완주·재접속 흐름 회귀 없음)
- **F3** `isSafeInternalPath` 가 백슬래시(`\`)·제어문자를 통과시켜,
  일부 브라우저가 `\\evil.com` 을 `//evil.com` 으로 정규화하면서 발생하던
  `/fileopen` 오픈 리다이렉트 우회 차단.

### Fixed (안정성 / 동작 결함)

- **mailer** `dialHost` 가 ctx 취소 시 `(nil, nil)` 을 반환해 후속
  `defer sender.Close()` 에서 nil panic 가능하던 결함 보강 (sender==nil 가드).
- **training** `TrainingCompleteHandler` 를 서버 권위 단독 완료판정으로 변경.
  클라이언트 watched/duration 자가증명 경로 제거, `video_progresses` +
  `videos.duration_seconds` 만 신뢰. 캠페인 완료 410, invalid video_id 400 추가.
- **#13** RP/LP 렌더 시 `Swal.fire` 호출은 있으나 `window.Swal` 미정의인
  stale 페이지에서 `ReferenceError` 가 나던 결함 보강. `ensureMiniSwal` 이
  미니 폴리필을 멱등 주입 (LP=renderPhishResponse, RP=RedirectPageHandler).
- **F2** `Result.UpdateGeo` 의 `log.Fatal` 제거. GeoIP DB 열기 실패 시
  `os.Exit` 로 피싱+어드민 서버가 동시 종료되던 가용성 결함을, 에러 반환 +
  호출부(setupContext)의 기존 graceful 처리(로그 후 계속)에 위임하도록 변경.
- **F4** `/media/{id}` 핸들러가 rid 의 transparency 접미사를 미제거하던
  비일관 보강. FileOpen/TrainingComplete 와 동일하게 `TrimSuffix` 적용.

### Tested (회귀 방지)

- `TestTrackVideoServerAuthoritative` 추가 — F1 위조 차단 + 정상 완주
  무회귀 + seconds_watched 상한 클램프를 동시 검증.

### Known Limitations

- stale RP/LP page body — 빌더 JS 개선이 DB 저장된 기존 RP/LP 본문에 미소급.
  운영 대응 = 관리 UI 학습 템플릿 재저장. 코드 트랙은 rc2+ 후보.
- dist 동기화 stale — `dist/*.min.js` 가 src 보다 과거. gulp 재빌드 필요.
  의존성 보안 패치와 함께 별도 트랙 (Phase 1.5) 에서 처리.
- 브랜딩 미적용 — 신규 브랜딩 확정·전면 적용은 rc3 에서 처리.

---

[Unreleased]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-rc2...HEAD
[1.0.0-rc2]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-rc1...v1.0.0-rc2
[1.0.0-rc1]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-beta2...v1.0.0-rc1
[1.0.0-beta2]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-beta1...v1.0.0-beta2
[1.0.0-beta1]: https://github.com/AegisAX/Sentinel/releases/tag/v1.0.0-beta1
