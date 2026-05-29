# Changelog

이 프로젝트의 모든 변경 사항은 이 파일에 기록됩니다.

[Keep a Changelog](https://keepachangelog.com/) 형식을 따르며,
이 프로젝트는 [Semantic Versioning](https://semver.org/spec/v2.0.0.html) 을 사용합니다.

## [Unreleased]

### Phase 3 — Branding (다음 트랙, rc4 대상)
- 브랜딩 적용 (이름/로고/색상) + X-Server 헤더
- `templates/base.html` Google Fonts CDN → 로컬 vendor 전환 (#55 flag-icons 연장선)
- dist 동기화 재빌드 (rc1 Known Limitations 해소)

### Phase 4 — Low (코드 정리)
- (운영 검증 후 결정)

### Phase 5+ — 출시 후 (v1.1 영역)
- TestAttachment 사전 결함 (testdata 옛 변수)
- `/videos/upload` 제거 + JS 를 `/api/videos/` 로 이전
- Email Template / Landing Page / Redirect Page 권한 체계 통일

---

## [1.0.0-rc3] - 2026-05-29

v1.0.0-rc2 출시 후 누적된 보안 결함 차단 4건 + 운영 검증 결함 수정 8건
+ UX/구조 개선 6건 + 신규 기능 3건을 묶은 RC 패치 릴리스. 신규 마이그레이션
2건 (#64 campaign_shares, #66 user_preferences). 다음 트랙은 브랜딩 적용.

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
- **#64** 캠페인 결과 화면 read-only 공유 기능. 캠페인 소유자가 다른 사용자에게
  자기 캠페인의 **결과 화면만** 볼 수 있는 권한을 부여한다. 공유받은 사용자
  (viewer) 는 결과/수강현황/국가Top10/Export 모두 그대로 보지만, Complete/
  Delete/공유/대상자 세부 타임라인/신고 토글 등 모든 변경 동작은 불가능하며
  관련 UI 도 가려진다. 권한 모델 자체는 손대지 않고(`EnforceViewOnly`/`Admin/User`
  2단계 그대로) 캠페인 단위의 grant 차원을 신규 추가해서, 기존 owner-only
  격리(#41/#58/#59/#61/#2 류)를 깨지 않는다.
  - 데이터: 신규 테이블 `campaign_shares(id, campaign_id, user_id, created_date)`
    + UNIQUE(campaign_id, user_id) + INDEX(user_id). SQLite/MySQL 마이그레이션
    `20260529000000_add_campaign_shares.sql`. 캠페인/사용자 삭제 시 cascade
    정리 (`DeleteCampaign`·`DeleteUser` 보강).
  - 판정 헬퍼: `models.CanViewCampaign(id, viewerUid)` 가 (1) 소유자 또는
    (2) campaign_shares 에 grant 존재 시 통과. 데이터 조회는 여전히 owner
    uid 로 수행 — 즉 권한 판정만 viewer uid, 데이터는 owner uid 라는 분리
    구조로 GORM 의 user_id 필터 불변식을 보존.
  - 읽기 API 게이트 (controllers/api/campaign.go): `/api/campaigns/{id}`
    (GET), `/results`, `/summary`, `/video_progress`, `/country_stats` 5개에
    viewer 통과 게이트 추가. DELETE/`/complete`/`/results/{rid}/report` 는
    owner-only 그대로 (viewer uid 로 `GetCampaign` 실패 → 자동 404).
  - 공유 관리 API (owner-only):
      `GET    /api/campaigns/{id}/shares`            — 현재 공유자 + 후보
      `POST   /api/campaigns/{id}/shares`            — `{"user_id": N}` 추가
      `DELETE /api/campaigns/{id}/shares/{uid}`      — 해제
    `/api/users/` 가 admin 전용이라 후보 사용자 목록도 이 GET 응답에 함께
    실어 비-admin owner 도 picker 를 채울 수 있게 함 (id + username 만).
  - 캠페인 목록·Dashboard 노출: `GetCampaignSummaries(uid)` 가 owned + shared
    union 을 반환. 각 summary 에 `is_owner` 와 `owner_username` 동봉.
    Dashboard 의 "Click Rate Over Time" 차트는 viewer 의 공유 캠페인도
    자동 포함됨.
  - 결과 화면 응답에 `is_owner` 동봉 (`/api/campaigns/{id}/results`) — 프런트가
    viewer/owner 모드를 분기. viewer 화면에서는 Complete/Delete/공유 버튼 숨김,
    결과 테이블 행의 ▶ caret 자체 비표시 + 행 펼치기 이벤트 차단 + 셀
    포인터 커서 제거(`.details-control` 클래스 미적용), 신고 컬럼은 아이콘만
    표시(onclick 미부착).
  - 공유 UI (owner 전용): 결과 화면 상단에 "공유" 버튼 + 모달
    (대상 사용자 선택 + 현재 공유자 목록 + 해제). campaign_results.html /
    campaign_results.js.
  - 목록 UI: 캠페인/Dashboard 행에 owner_username 부기 (viewer 시점에서만,
    날짜 줄 옆 회색 작은 글씨). 캠페인 목록 Actions 컬럼 헤더 명시 추가 +
    좌측 정렬로 Dashboard 와 통일.

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
- **#60** 캠페인 삭제 시 `video_progresses` 가 함께 삭제되지 않아 고아 행이
  남던 결함 보강. `video_progresses` 에는 `campaign_id` 가 없고 `result_id`
  로만 results 와 연결되는데, `DeleteCampaign` 이 results/events/maillogs 만
  지우고 video_progresses 를 누락해, 캠페인을 지울 때마다 result 와 끊긴
  고아 progress 가 누적되었다. `DeleteCampaign` 에 `result_id IN (SELECT id
  FROM results WHERE campaign_id = ?)` 서브쿼리 삭제를 results 삭제 *앞*
  단계로 추가 (순서가 바뀌면 result_id 링크가 끊겨 삭제 불가). 기존 고아
  행은 운영 DB 에서 정리.
- **#63** 캠페인 결과 Export 의 'Raw Events' 가 'Events(CSV)' 와 동일한
  파일명(`{캠페인명} - Events.csv`)으로 저장되어 두 파일이 구분되지 않던
  결함 보강. Raw Events(타임라인 원본 덤프) 의 파일명을 `{캠페인명} -
  Raw Events.csv` 로 분리. Events(CSV, 정리된 per-수신자) 는 기존 파일명
  유지. `static/js/src/app/campaign_results.js` 의 `exportAsCSV` 정리 +
  공통 다운로드 헬퍼 `downloadCSV` 로 추출.
- **#65** 완료된 캠페인(`CampaignComplete`)에서 결과 화면의 신고 토글 ON 시
  클라이언트에 404 알림이 노출되던 결함 보강. 기존 `toggle_report` 는 ON
  분기에서 phishing 서버 `/report?rid=` 를 fetch 했는데, `/report` 핸들러의
  CampaignComplete 가드에 막혀 404 가 반환됐다. 완료 후에도 사후 신고
  반영이 가능해야 하는 운영 요구를 만족시키기 위해, ON 경로를 OFF 와
  대칭인 admin API 로 통일했다 — 신규 `POST /api/campaigns/{id}/results/
  {rid}/report` 라우트 + owner-only 핸들러 `CampaignResultReport` +
  `models.ReportResult(rid)` (멱등 — 이미 reported 면 nil 반환,
  `HandleEmailReport(EventDetails{})` 호출로 Reported 이벤트 + reported
  플래그 동시 처리). 토글 done 콜백은 `refresh()` 가 완료 캠페인에서
  `doPoll=false` 가드로 즉시 리턴되는 점을 우회해 `poll()` 만 직접 호출
  + 인디케이터 수동 토글 — 자동 폴링 루프는 그대로(완료 캠페인은 더 이상
  이벤트 없음). viewer 는 owner-only 그대로 (`GetCampaign(cid, uid)` 검증
  으로 자동 404).

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
- **#63** 캠페인 결과 Export 의 'Results' CSV 에 `Trained`·`Watch Time`
  두 컬럼 추가. Trained 는 status 를 바꾸지 않고 timeline 에만 기록되어
  기존 Results CSV(result 원본 덤프)에서 누락되었고, 영상 시청 시간은
  `/api/campaigns/{id}/video_progress` 에만 있어 포함되지 않았다. timeline
  의 Trained 이벤트로 수강 완료(`Yes`)를 판정하고, video_progress 의
  `seconds_watched` 를 email 1:1 매핑해 `Watch Time`(`m:ss`)으로 출력.
  수강 데이터 조회 실패 시에도 결과는 정상 내보냄.
- **#66** Dashboard 와 Campaigns 페이지 통합 (옵션 C). `/campaigns` 라우트
  + nav.html 의 Campaigns 메뉴 항목 + campaigns.html / campaigns.js 모두
  제거하고, Dashboard 가 캠페인 관리의 단일 진입점이 되도록 통합. Dashboard
  에는 기존 차트/도넛/캠페인 목록 위에 영구 필터 + 검색 동기화 + 캠페인
  생성 흐름을 모두 흡수했다.
  - **영구 필터** : 신규 테이블 `user_preferences(user_id, dashboard_campaign_filter,
    modified_date)` + 마이그레이션 `20260530000000_add_user_preferences.sql`
    (SQLite/MySQL). 사용자 삭제 시 cascade (`DeleteUser` 보강). 일반화된
    `/api/users/me/preferences` 엔드포인트 (GET/PUT) 로 향후 다른 환경설정도
    같은 곳에 확장 가능. localStorage 가 아닌 DB 저장 — 다른 PC/브라우저에서
    접속해도 동일 필터 유지.
  - **DataTables hidden 컬럼 + search hook 패턴** : 캠페인 id 를 row 의 11번째
    hidden 컬럼에 저장하고, `$.fn.dataTable.ext.search` 에 등록한 필터 함수가
    `data[10]` 으로 직접 읽어 필터링. 정규식 HTML 파싱 의존성 제거. `searchable:
    false` 는 search hook 의 data 인자에서 빈 문자열로 들어와 매칭이 깨지므로
    의도적으로 `searchable: true` 유지.
  - **차트 동기화** : DataTables 의 `draw.dt` / `search.dt` 이벤트로 가시 행에서
    캠페인 id 를 모아 도넛 + Click Rate Over Time 차트를 매번 재집계. 필터 +
    DataTables 검색이 모두 적용된 결과로 차트가 항상 일관.
  - **toolbar** : `Show N entries` 우측에 [+ New Campaign] [⚙ 캠페인 선택]
    버튼을 DataTables 의 length 영역에 동적으로 주입 (campaign_results.js 의
    컬럼 토글 패턴과 동일).
  - **캠페인 선택 모달** : 다중 체크박스 + 검색 (이름/소유자/상태) + 세부정보
    토글 (Launch Date / Send By / Stats 미니 요약 + [View Results →]) +
    선택 카운트 ("N / M 선택됨"). 빈 선택 = "전체 표시" 로 간주 (UI 와 동일 의미).
  - **viewer 동작** : shared 캠페인의 Copy/Delete 는 미표시가 아닌 `disabled`
    상태로 노출 — "왜 안 보이지?" 혼동 제거, 권한 부재를 명시. 호버 시 안내
    tooltip ("공유받은 캠페인은 복제/삭제할 수 없습니다").
  - **시각/색상 정리** :
      Actions 의 [View Results / Copy / Delete] 색상을 의미에 맞게 분리
      (info / success / danger). 비표준 `btn-blue` 클래스 의존 제거 →
      Refresh = btn-info, Complete = btn-success 로 교체. 공유 버튼은
      btn-warning (주황) — 권한 부여의 보안적 의미 + 인접 Refresh 와 색 구분.
      Actions 버튼 한 줄 nowrap + 4px 미세 간격.
      `tooltip placement: left → top` — Actions 가 마지막 컬럼이라 좌측
      tooltip 이 다른 컬럼 데이터를 가리는 결함 보정.
  - **Name 컬럼 최후 압축** : 좁은 viewport 에서 숫자/Status/Actions 컬럼은
    nowrap 유지 + Name 컬럼만 wrap + min-width 220px. 가장 의미 있는 컬럼이
    가장 마지막에 줄어들도록.
  - **Status hover tooltip 제거** : 같은 행에 모든 stat (Sent/Opened/...) 과
    launch_date 가 표시되어 tooltip 의 정보가 중복. 라벨만 단순 표시. Name
    아래 날짜를 `created_date` → `launch_date` 로 일원화 (운영자에게 더 의미
    있는 시각).
  - **New Campaign 모달 흐름 개선** : 캠페인 생성 전제조건(Email Templates →
    Landing Pages → Sending Profiles → Groups) 을 위→아래 순으로 순차 검사.
    첫 부족 항목 발견 시 그 안내만 표시하고 이후 단계는 검사 안 함. 누락 항목
    이후의 입력 필드는 모두 숨김 + Launch Campaign 비활성화 + "이동" 버튼으로
    해당 어드민 페이지로 직접 안내. 모달 재오픈 시 상태 초기화 (`resetSetupOptionsUI`).
  - **Send By 의 Go time.Time 제로값 표시 결함** : `0001-01-01T00:00:00Z` 가
    truthy 라 단순 if-truthy 만으로는 `-` 로 치환되지 않던 결함을 정규식 가드로
    차단. 백엔드 nullable 화는 Phase 5+ 백로그.
  - **라벨 한국어 통일** : Launch Date → "발송 시작", Send Emails By →
    "발송 마감 (선택)" / Send By → "발송 마감". 영문 툴팁도 한국어로 정확한
    의미 명시 ("지정 시, Sentinel 은 발송 시작 시각과 이 시각 사이에 균등하게
    메일을 발송합니다"). 단어 자체의 정확성은 향후 i18n 도입 트랙에서 재검토.
  - **Results 페이지 잔여 정리** : "Back" 버튼의 링크 `/campaigns` → `/`,
    deleteCampaign 후 redirect `/campaigns` → `/`. campaigns.html / campaigns.js
    잔재 제거.

### Security (보안 결함 차단)

- **#58** RedirectPage(`/rp/{id}`) 접근에 선행 행동 게이트 추가.
  캠페인 설계 의도(Submitted/Executed 한 수신자만 교육 영상 RP 로 이동)와
  달리, rid 만 유효하면 Opened/Clicked 단계의 수신자나 메일 클라이언트·보안
  게이트웨이의 링크 프리페치가 `/rp/{id}?rid=` 에 직접 도달해 RP 가 렌더되고,
  그 결과 RP 전용 영상의 수강 progress·Trained 이벤트가 생성돼 수강 현황에
  노이즈가 섞이던 결함. `RedirectPageHandler` 의 #41 매핑 검증 뒤에
  `result.Status == Submitted || result.Executed` 게이트를 추가해 미충족 시
  404 반환. status 는 Submitted 이후 낮아지지 않고 Executed/Trained 는
  status 를 건드리지 않으므로, 정상 Submitted/Executed → RP 흐름과 Trained
  수신자의 RP 재방문은 회귀 없이 통과한다. (#41 cross-campaign 자산 접근
  차단의 자매 결함)
- **#59** `/track/video`·`/api/training/complete` 직접 호출 우회 차단 (#58
  follow-up). #58 은 RP '렌더'만 막을 뿐, 두 핸들러는 rid + video_id 만으로
  직접 POST 호출이 가능해, 정상 브라우저 흐름(LP/RP 렌더)을 거치지 않고도
  RP 전용 영상의 수강 progress·Trained 이벤트를 위조할 수 있었다. 두 핸들러
  공유 헬퍼 `videoActionAllowed(result, campaign, videoID)` 신규 — (1) 요청
  video_id 가 캠페인 LandingPage 임베드 영상이면 통과(Clicked 단계 정상 시청),
  (2) 캠페인 RedirectPage 영상이면 #58 과 동일하게 `Submitted || Executed`
  요구, (3) 캠페인에 연결되지 않은 video_id 는 거부. `TrackVideo` 는 404,
  `TrainingCompleteHandler` 는 403 반환. 두 핸들러 모두 캠페인 조회 실패 시
  중단하도록 보강(게이트가 campaign 에 의존). 정상 흐름·#58 통과 케이스는
  회귀 없음.
- **#61** 단건 GET `/api/videos/{id}` 와 썸네일 `/videos/thumb/{id}` 에
  own-or-public 불변식 적용 (#2 follow-up). `/videos/stream`·`/media` 는 #2
  에서 소유자 아님 + 비공개면 404 로 막았으나, 단건 메타데이터 GET 과 썸네일
  핸들러는 `models.GetVideo(id)` 결과를 소유권 검사 없이 반환해, 로그인한 다른
  사용자가 ID 추측으로 남의 영상 메타데이터(`file_path` 서버 경로 포함)·썸네일을
  조회할 수 있던 cross-tenant IDOR 잔여 경로가 있었다. 두 핸들러에 목록
  (`GetVideosForUser` 의 `user_id = ? OR is_public`)·StreamVideo 와 동일한
  게이트(본인 소유 또는 is_public, 그 외 존재 노출 방지 위해 404)를 추가.
- **#62** Admin 응답에 `X-Content-Type-Options: nosniff` 추가
  (`ApplySecurityHeaders`). MIME 스니핑 기반 공격 표면을 축소하는
  defense-in-depth. `/media`·`/videos/stream` 등 개별 핸들러는 이미 nosniff
  를 설정했으나 admin 공통 미들웨어에는 누락돼 있었다 (#18 Referrer-Policy 와
  동일 위치).

### Tested (회귀 방지)

- `TestVideoGetCrossTenant` 추가 (#61) — 단건 GET 이 비소유·비공개 영상은
  404, 소유자·is_public 영상은 200 임을 검증.
- `TestTrackVideoServerAuthoritative` (rc1 F1 가드) 셋업 복구 — #59 의
  `videoActionAllowed` 게이트 도입 이후, 캠페인 LP/RP 에 연결되지 않은 영상으로
  `/track/video` 를 호출하던 기존 테스트 셋업이 게이트에 막혀 실패하게 되었다.
  영상을 캠페인 LandingPage 에 연결(+결과를 Clicked 로 진전)하도록 시나리오를
  현실화. 테스트 전용 변경, 제품 동작 영향 없음.

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

[Unreleased]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-rc3...HEAD
[1.0.0-rc3]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-rc2...v1.0.0-rc3
[1.0.0-rc2]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-rc1...v1.0.0-rc2
[1.0.0-rc1]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-beta2...v1.0.0-rc1
[1.0.0-beta2]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-beta1...v1.0.0-beta2
[1.0.0-beta1]: https://github.com/AegisAX/Sentinel/releases/tag/v1.0.0-beta1