# Changelog

이 프로젝트의 모든 변경 사항은 이 파일에 기록됩니다.

[Keep a Changelog](https://keepachangelog.com/ko/1.1.0/) 형식을 따르며,
이 프로젝트는 [Semantic Versioning](https://semver.org/spec/v2.0.0.html) 을 사용합니다.

## [Unreleased]

### Phase 3 — Medium (브랜딩/일관성, v1.0.0-rc1 대상)
- (예정)

### Phase 4 — Low (코드 정리, v1.0.0-rc2 대상)
- (예정)

### Phase 5+ — 출시 후 (v1.1 영역)
- TestAttachment 사전 결함 (testdata 옛 변수)
- `/videos/upload` 제거 + JS 를 `/api/videos/` 로 이전
- Email Template / Landing Page / Redirect Page 권한 체계 통일
- 의존성 보안 패치 (Phase 1.5: gulp/webpack 메이저 업그레이드, npm/yarn 정책)

---

## [1.0.0-beta2] - 2026-05-11

v1.0.0-beta1 운영 검증 중 발견된 결함을 수정한 패치 릴리스.

### Fixed (운영 검증 — 발견된 결함)

- **#9** `/media/{id}` 인증 검증이 LandingPage `video_id` 만 검사하던 결함 보강.
  RedirectPage 에만 영상을 임베드한 정상 수신자가 404 를 받던 케이스 해결.
  `models.IsVideoLinkedToUser` 헬퍼 신규 추가 — result 소유자의 LP/RP 어느 한
  곳에라도 video_id 가 연결되어 있으면 통과시킴. (Phase 2 #7 follow-up)
- **#10** 영상 수강 완료 후 `[수강 완료 확인]` 버튼 클릭 전 새로고침/재접속 시
  완료 상태가 사라지던 결함 보강. `/track/video/progress` 응답의 `completed:true`
  플래그를 인지해 영상 영역을 안내 박스로 교체 + 완료 버튼 자동 활성화.
  `alreadyCompleted` 플래그로 클라이언트 측 90% 시청률 가드 우회.
- **#11** 영상 종료(`ended`) 시 video controls 를 제거해 재시작 차단.
  완료 상태에서 사용자가 처음부터 재생해도 "수강 완료 확인" 이 활성 유지되던
  UX 혼동 해소.
- **#12** `SendTestEmail` 이 mailer 큐의 `ErrorChan` 을 무한 대기하던 결함 보강.
  10초 타임아웃 + 명확한 에러 메시지("SMTP 서버 응답이 없습니다. 호스트/포트/
  방화벽을 확인하세요. (10초 타임아웃)") 반환. 이전엔 SMTP 서버 응답 없을 시
  최대 5분 대기 후에야 사용자에게 피드백.

---

## [1.0.0-beta1] - 2026-05-08

GoPhish 0.12.1 fork 인 Sentinel 의 첫 베타 출시. 출시 블로커와 보안 결함을
모두 수정했으며, 사이버보안 인식 교육 영상 임베드 기능을 추가했다.

### Added (Sentinel 신규 기능)
- 사이버보안 인식 교육 영상 임베드 기능
  - `/api/videos/` (REST API), `/videos/upload` (멀티파트 업로드)
  - `/media/{id}` (영상 스트리밍), `/videos/thumb/{id}` (썸네일)
  - LandingPage / RedirectPage 의 `video_id` 컬럼 + CKEditor 영상 삽입 워크플로
- RedirectPage 객체: 캠페인 종료 후 별도 교육 페이지 라우팅
- Report Form (`/report-form`): 피싱 의심 신고 페이지
- Attachment Executed 이벤트: 첨부 파일 실행 (very high risk, 빨간색)

### Fixed (출시 블로커 — Phase 1)
- **#1** SQLite/MySQL 마이그레이션 `smtp_headers` → `headers` 테이블명 정정
  (잘못된 테이블명으로 신규 설치 100% 실패하던 버그)
- **#2** `controllers/phish.go` `/report-form` 의 `Fprintf` 포맷 깨짐 수정
  (`width:90%` 가 `width:90%!;(MISSING)` 로 출력되던 버그)
- **#3** `models/models_test.go` 중복 `Department` 필드 정정
- **#3.5** `TestNoRecipientID` 를 Sentinel 의 `/` → `/report-form` 302 리다이렉트
  동작에 맞게 갱신
- **#4** Dockerfile golang 1.15.2 → 1.26 업그레이드, `go.mod` 의 불필요한
  `toolchain` 라인 제거
- **#4.5** 미사용 `nosurf` 의존성 제거

### Security (보안 결함 — Phase 2)
- **#5** JSON `PUT /api/videos/{id}` 의 데이터 파괴 + `user_id` 양도 취약점 차단.
  GORM v1 `db.Save()` 가 모든 컬럼을 zero-value 로 덮어쓰던 결함을 화이트리스트
  패턴 (name/description/is_public 만 갱신) 으로 수정.
- **#6** `/videos/upload` 에 `ModifyObjects` 권한 명시 검사 추가 (defense-in-depth).
- **#7** `/media/{id}` rid 기반 캠페인-영상 연결 검증.
  외부 ID 추측만으로 다른 사용자의 영상이 노출되던 cross-tenant 결함 차단.
  Go Media 핸들러에서 `rid → Result → Campaign → Page.VideoId` 경로로 검증하며,
  마이그레이션이 기존 페이지의 영상 src 에 `?rid={{.RId}}` 토큰을 자동 추가.
- **#8** `/videos/thumb/{id}` 인증 추가 (admin 전용 GET 라우트가 무인증 노출되던 결함).

### Technical
- 마이그레이션 신규 1건: `20260507000000_add_rid_to_video_src.sql`
  (SQLite + MySQL, self-closing `<source>` 태그의 두 형태(`/>`, `" />`) 모두 처리,
  멱등성 보장, Down 으로 완전 롤백 가능)

### Known Limitations
- Phase 3 (브랜딩/일관성) 미적용 — `v1.0.0-rc1` 에서 처리 예정
- Phase 4 (코드 정리) 미적용 — `v1.0.0-rc2` 에서 처리 예정
- 의존성 보안 패치 (gulp 4, webpack 4 잔존) — 별도 트랙 (Phase 1.5)

[Unreleased]: https://github.com/AegisAX/Sentinel/compare/v1.0.0-beta1...HEAD
[1.0.0-beta1]: https://github.com/AegisAX/Sentinel/releases/tag/v1.0.0-beta1
