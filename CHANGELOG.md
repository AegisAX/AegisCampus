# Changelog

이 프로젝트의 모든 변경 사항은 이 파일에 기록됩니다.

[Keep a Changelog](https://keepachangelog.com/ko/1.1.0/) 형식을 따르며,
이 프로젝트는 [Semantic Versioning](https://semver.org/spec/v2.0.0.html) 을 사용합니다.

## [Unreleased]

### Added (신규 기능)

- **#54** 캠페인 결과 목록에 접속 국가(Country) 컬럼 + 컬럼 표시 토글 신규.
  `result.ip` 기준으로 Top10 / 결과 목록 Country / Targets Map bubble 의
  국가 판정을 단일 기준으로 통일. `models/result.go` 에 `resolveCountry`
  (mmdb, ip)→(name, iso), `AttachCountries` (results IP 캐시 in-place),
  `GetCountryStatsByCampaign` (result.ip distinct 집계) 추가. GeoIP DB 는
  `static/db/geolite2-city.mmdb`. Country 컬럼은 국기 아이콘 + 국가명을
  한 줄(`white-space:nowrap`)로 표시. '컬럼 ▾' 드롭다운으로 컬럼 표시
  토글 (localStorage `sentinel.resultCols`, 전 캠페인 공통, 기본 숨김
  Email/Position/Status).
- **#55** flag-icons 7.5.0 로컬 vendor 포함 (사내망/오프라인 운영 원칙).
  `static/css/flag-icons/` (CSS + 4x3/1x1 flags, 542 SVG). `templates/
  base.html` 에 별도 `<link>` 로 로드, `sentinel.css` 에 concat 하지
  않음 (상대경로 깨짐 방지). 외부 CDN 의존 제거의 첫 적용.

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
- **#49** Dashboard 의 기존 "Phishing Success Overview" 차트 (캠페인당 점 1개,
  X축 = 캠페인 생성일, Y축 = 클릭률) 가 캠페인 비교에는 적합하지 않고
  Recent Campaigns 표와 정보 중복이라 사실상 추세 의미가 없던 결함 보강.
  캠페인별 누적 클릭률 라인 차트로 교체 — X축 = launch 후 경과 시간
  (`Xh Ym`), Y축 = 누적 클릭률 (`ceiling: 100`, `softMax: 10`). 백엔드
  `CampaignSummary` 응답에 `click_timeline` 필드 추가 (`getCampaignClickTimeline`
  헬퍼 — distinct 클릭자 시각 시리즈, SQLite `MIN(time)` 의 string affinity
  파싱). legend 토글 시 series 색 ↔ 회색 전환, hover tooltip 에 캠페인명 +
  절대 시각 (로컬) + 경과 시간 + 클릭률 4줄 병기.
- **#50** 도넛 차트 비율이 `Math.floor` 로 정수 잘림되어 16.5% 가 16% 로
  표시되고 누적합이 100% 가 안 맞던 정밀도 결함 보강. `Math.round(... * 1000) / 10`
  로 소수점 1자리 유지. Dashboard 와 캠페인 상세 페이지 양쪽 동일 적용,
  tooltip 에 `Highcharts.numberFormat(value, 1)` 적용.
  - **#57** 수강 현황에 동일 수신자가 중복 표시되던 결함 보강.
  `VideoProgress.Save()` 의 자연키 (user_id, result_id, video_id) upsert
  가 First→Save 사이 race 로 동시 `/track/video` 비콘에서 중복 INSERT
  되던 결함 차단. UNIQUE 인덱스 `idx_video_progresses_unique_urv` 추가
  (SQLite/MySQL 마이그레이션) + `Save()` 가 UNIQUE 위반 시 기존 행
  재조회 후 UPDATE 재시도. 기존 중복 행은 운영 DB 에서 정리.

### Changed (UX / 일관성)

- **#51** 캠페인 상세 페이지의 '결과 목록' / '수강 현황' 탭 시각적 통일.
  중복 헤딩 `<h2>Details</h2>` 제거 (탭 라벨과 의미 중복). 수강 현황
  테이블에 DataTable 적용 (Show entries / Search / 페이징 / 정렬). 두
  탭 상단 여백 (`margin-top:15px`) 일치. 수강 현황 테이블 class 를 결과
  목록과 동일하게 통일 (`table-condensed table-bordered table-hover` →
  `table`). `col-md-12` 래퍼 제거로 테이블 가로 폭 일치.
- **#52** 수강 현황의 '완료' 컬럼이 진행률 100% 또는 ended 이벤트만으로도
  완료 표시되어, 실제 [수강 완료 확인] 버튼을 누르지 않은 케이스를 식별할
  수 없던 결함 개선. `VideoProgressSummary` 에 `trained` 필드 신규
  (`events.message = 'Trained'` EXISTS 서브쿼리). 프론트는 `row.completed`
  대신 `row.trained` 로 배지 렌더. 컬럼 헤더 '완료' → '수강 완료' 로
  의미 명확화. 시청 완료 여부는 기존 진행률 컬럼으로 충분히 식별 가능.
- **#53** Targets Map (캠페인 결과 지도) 시각/상호작용 개선. 점 색을
  주황 그라데이션 + 그림자로 변경, 줌 동작 추가 (wrapper `g` + bubble
  detach/attach), 국가별 Top10 표시, 하단 여백 보정 (`padding-bottom:40%`).
- **#56** 캠페인 상세 페이지 '국가별 접속 Top 10' 리스트의 국가명과
  접속 수 사이 여백이 컬럼 폭에 따라 과하게 벌어지던 UX 결함 보정.
  `#countryTopList` 폭을 컨테이너의 80% 로 제한 (`width: 80%`). 숫자는
  `margin-right: auto` 유지로 우측 정렬 그대로.

### Phase 4 — Low (코드 정리, 미정)
- (예정)

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
