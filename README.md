<p align="center">
  <img src="static/images/aegiscampus_mark.svg" width="80" alt="" />
</p>

<h1 align="center">AegisCampus</h1>

<p align="center">
  Cybersecurity training & phishing simulation, in one platform.<br/>
  악성메일 모의훈련과 사이버보안 교육을 하나의 플랫폼에서
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-1.0.0--rc8-3F3D7A" alt="Version" />
  <img src="https://img.shields.io/badge/license-MIT-3F3D7A" alt="License" />
  <img src="https://img.shields.io/badge/Go-1.26%2B-3F3D7A" alt="Go" />
  <img src="https://img.shields.io/badge/Node-24%20LTS-3F3D7A" alt="Node" />
</p>

---

<p align="center">
  <img src="static/images/aegiscampus_infographic.svg" width="800" alt="AegisCampus system overview" />
</p>

## 개요 / Overview

**AegisCampus** 은 피싱 시뮬레이션과 보안 인식 교육 영상을 하나의 도구에서 통합 운영할 수 있는 자체 호스트 플랫폼입니다. "잘못된 클릭 → 즉시 교육 영상 → 수강 완료 추적" 흐름으로, 시뮬레이션 그 자체로 끝내지 않고 학습 효과까지 캠페인 보고서에 통합합니다.

*AegisCampus is a self-hosted platform that combines phishing simulation with embedded video training. Beyond simulating attacks, it tracks whether employees who fell for a campaign actually watched and completed the assigned training video — the `Trained` event joins the existing `Sent → Opened → Clicked → Submitted` timeline, giving security teams a complete picture from compromise to remediation.*

## 핵심 특징 / Key features

- **교육 영상 임베드** — MP4 업로드 후 랜딩 페이지 또는 리다이렉트 페이지에 임베드. 수신자별 시청 진행률 및 수강 완료 (`Trained` 이벤트) 추적
- **리다이렉트 페이지** — 캠페인 클릭 후 보여줄 자체 교육 페이지. 단순 `redirect_url` 을 영상 임베드 가능한 편집 가능 페이지로 확장
- **신규 추적 이벤트** — `Executed` (첨부 실행) + `Trained` (교육 영상 수강 완료) 두 가지 이벤트 추가
- **피싱 신고 폼** — 수신자 친화적 `/report-form` 엔드포인트. `rid` 토큰이 없거나 만료되어도 이메일 + 제목 fallback 매칭
- **한국 기업 환경 호환** — `name` / `department` 스키마, RFC 2047 제목 인코딩, RFC 5987 첨부 파일명, FQDN 기반 `Message-ID` (Gmail 5.7.1 회피)
- **보안 강화** — cross-tenant 영상 접근 차단, 첨부 업로드에 `ModifyObjects` 권한 강제, JSON `PUT /api/videos/{id}` 화이트리스트 컬럼

자세한 변경 내역은 [CHANGELOG.md](CHANGELOG.md) 참조.

---

## 요구 사항 / Requirements

요구 사항은 **개발(빌드)** 과 **운영(실행)** 으로 나뉩니다. 운영에는 빌드 도구가 필요 없습니다.

### 개발 환경 (소스 빌드 시)

- **Go** 1.26.0 이상 — go.mod 의 `toolchain go1.26.4` 가 빌드 toolchain 을 고정합니다. `GOTOOLCHAIN=auto` (기본값) 환경에서는 빌드 시 1.26.4 가 자동 조달됩니다.
- **Node.js 24 LTS** + **npm 11** — 프론트엔드 자산(JS/CSS) 빌드 전용입니다. 이미 빌드된 `static/*/dist` 산출물이 저장소에 포함되어 있어, **JS/CSS 소스를 수정하지 않는 한 Node 없이도** Go 빌드·실행이 가능합니다.
- **SQLite3** (기본) 또는 **MySQL**

### 운영 환경 (바이너리 실행 시)

- 빌드된 `aegiscampus` 바이너리 + `static/` + `templates/` + `db/` + `VERSION` + `config.json`
- **ffmpeg** + **ffprobe** — 교육 영상 썸네일 생성 및 길이 감지에 필요 (PATH 또는 환경변수로 경로 지정). 없으면 기동 시 stderr 에 경고가 출력됩니다.
- **SQLite3** (기본) 또는 **MySQL**
- Go·Node 런타임은 **불필요** (단일 바이너리 실행)

---

## 개발 환경 구축 / Development setup

```
git clone https://github.com/AegisAX/AegisCampus.git
cd AegisCampus
```

### 1) Go toolchain

```
# go.mod 의 toolchain(go1.26.4)이 자동 조달되는지 확인
go build -o /dev/null . 2>/dev/null || true
go version    # 시스템 go 가 1.26.x 미만이어도, 빌드 시 1.26.4 자동 사용
```

> 인터넷이 차단된 환경에서는 `GOTOOLCHAIN=auto` 가 toolchain 을 내려받지 못하므로, 시스템 Go 를 1.26.4 로 직접 설치하세요.

### 2) Node 24 LTS (프론트엔드 빌드용)

[nvm](https://github.com/nvm-sh/nvm) 사용 권장:

```
nvm install 24
nvm use 24
node --version    # v24.x
npm --version     # 11.x
```

### 3) 의존성 설치

```
npm ci    # package-lock.json 기준 정확 재현 설치
```

---

## 빌드 / Build

### 전체 빌드

```
# 1) 프론트엔드 자산 (JS 압축 + CSS 번들)
npm ci
npx gulp build

# 2) 백엔드 바이너리
go build -ldflags="-s -w" -trimpath -o aegiscampus .
```

> 바이너리 이름은 반드시 `-o aegiscampus` (소문자)로 지정하세요. go.mod 모듈명이 대문자라 `-o` 없이 빌드하면 대문자 바이너리가 생성됩니다.

### 변경 범위별 빌드

불필요한 재빌드를 피하기 위해 변경한 영역만 빌드합니다.

| 변경한 것 | 필요한 빌드 |
|---|---|
| Go 파일만 (`*.go`) | `go build` 만 (gulp 생략 가능) |
| JS/CSS 소스 (`static/*/src`) | `npx gulp build` 필수 (dist 는 src 와 같은 커밋에 포함) |
| 템플릿만 (`templates/*.html`) | 빌드 둘 다 생략 (런타임 로드) — `templates/` 만 반영 |

---

## 테스트 / Test

커밋·푸시 **이전에 반드시** 전체 테스트를 통과시킵니다.

```
go test ./...
```

포맷 검사(gofmt)도 함께 확인합니다:

```
diff -u <(echo -n) <(gofmt -d .)
```

> CI(GitHub Actions)는 push/PR 마다 Go 빌드 + 프론트엔드 빌드 + 포맷 검사 + `go test ./...` 를 실행합니다.

---

## 실행 및 설정 / Run & Configuration

### 첫 실행

```
./aegiscampus
```

첫 실행 시 초기 admin 비밀번호가 로그에 출력됩니다. 브라우저에서 `https://localhost:3333/` 접속 후 로그인하세요.

### config.json

`config.json` 으로 admin 서버(`:3333`), phishing 서버(`:8088`), DB 드라이버, TLS, 신뢰 출처(`trusted_origins`)를 관리합니다.

```json
{
  "admin_server": {
    "listen_url": "127.0.0.1:3333",
    "use_tls": true,
    "trusted_origins": []
  },
  "phish_server": {
    "listen_url": "0.0.0.0:8088",
    "use_tls": false
  },
  "db_name": "sqlite3",
  "db_path": "aegiscampus.db"
}
```

> 리버스 프록시 뒤에서 도메인으로 접속하는 경우 `admin_server.trusted_origins` 에 해당 도메인을 추가해야 합니다. (아래 [리버스 프록시 / 도메인 운영](#리버스-프록시--도메인-운영) 참조)

### 환경변수

| 변수 / Variable | 기본값 / Default | 설명 / Purpose |
|---|---|---|
| `AEGISCAMPUS_FFMPEG` | `ffmpeg` | ffmpeg 바이너리 경로 |
| `AEGISCAMPUS_FFPROBE` | `ffprobe` | ffprobe 바이너리 경로 |
| `AEGISCAMPUS_MAX_VIDEO_BYTES` | `524288000` (500 MB) | 영상 업로드 상한 (예: `1073741824` = 1 GB) |
| `AEGISCAMPUS_MSGID_DOMAIN` | (envelope sender → SMTP from → hostname 순 폴백) | `Message-ID` 헤더용 FQDN (Gmail 발송 시 권장) |
| `AEGISCAMPUS_EHLO_DOMAIN` | (FromAddress → hostname 순 폴백) | SMTP `EHLO` 인사말 FQDN. 미설정 시 일부 수신 서버가 거부할 수 있음 |
| `AEGISCAMPUS_INITIAL_ADMIN_PASSWORD` | (자동 생성) | 첫 부팅 시 초기 admin 비밀번호 |
| `AEGISCAMPUS_INITIAL_ADMIN_API_TOKEN` | (자동 생성) | 첫 부팅 시 초기 admin API 토큰 |

---

## 운영 배포 / Deployment (binary)

빌드 디렉터리에서 빌드한 산출물을 운영 디렉터리(`~/AegisCampus`)로 배포하는 절차입니다. **운영 DB(`aegiscampus.db`)와 `config.json` 은 보존**합니다.

```
# 1) 빌드 (개발 디렉터리에서)
npx gulp build
go build -ldflags="-s -w" -trimpath -o aegiscampus .
go test ./...

# 2) 실행 중인 프로세스 종료 → 잠시 대기
ps -ef | grep "./aegiscampus" | grep -v grep | awk '{print $2}' | xargs -r kill
sleep 2

# 3) 운영 디렉터리로 복사 (config.json·*.db 미포함)
cp -arv aegiscampus db/ static/ templates/ VERSION ~/AegisCampus/

# 4) 재시작
cd ~/AegisCampus && nohup ./aegiscampus > aegiscampus.log 2>&1 & disown
sleep 2 && tail -10 ~/AegisCampus/aegiscampus.log
```

> **순서 주의:** 반드시 *종료 → 대기 → 복사 → 재시작* 순입니다. 복사를 먼저 하면 사용 중인 바이너리에 대해 부분 배포가 발생할 수 있습니다.
> `cp` 대상에 `config.json` 과 `*.db` 가 포함되지 않습니다 — 운영 설정과 데이터가 보존됩니다. 재시작 로그에서 마이그레이션 적용 결과를 확인하세요.

---

## Docker 운영 / Docker

### 빌드 & 실행

```
docker build -t aegiscampus .
docker run -d --name aegiscampus \
  -p 3333:3333 -p 8088:8088 \
  aegiscampus
```

컨테이너의 설정은 **환경변수로 주입**합니다 (entrypoint `docker/run.sh` 가 `config.json` 을 런타임에 갱신).

| 환경변수 | config.json 매핑 | 설명 |
|---|---|---|
| `ADMIN_LISTEN_URL` | `admin_server.listen_url` | admin 서버 바인드 주소 (예: `0.0.0.0:3333`) |
| `ADMIN_USE_TLS` | `admin_server.use_tls` | admin TLS 사용 여부 (`true`/`false`) |
| `ADMIN_CERT_PATH` / `ADMIN_KEY_PATH` | `admin_server.cert_path` / `key_path` | admin TLS 인증서/키 경로 |
| `ADMIN_TRUSTED_ORIGINS` | `admin_server.trusted_origins` | 신뢰 출처 도메인 (콤마 구분, 예: `campus.aegisax.com`) |
| `PHISH_LISTEN_URL` | `phish_server.listen_url` | phishing 서버 바인드 주소 |
| `PHISH_USE_TLS` | `phish_server.use_tls` | phishing TLS 사용 여부 |
| `PHISH_CERT_PATH` / `PHISH_KEY_PATH` | `phish_server.cert_path` / `key_path` | phishing TLS 인증서/키 경로 |
| `CONTACT_ADDRESS` | `contact_address` | 투명성 헤더용 연락처 주소 |
| `DB_NAME` | `db_name` | DB 드라이버 (`sqlite3` / `mysql`) |
| `DB_FILE_PATH` | `db_path` | DB 파일 경로 |

```
# 예: 도메인 운영 + 신뢰 출처 지정
docker run -d --name aegiscampus \
  -p 3333:3333 -p 8088:8088 \
  -e ADMIN_LISTEN_URL=0.0.0.0:3333 \
  -e ADMIN_TRUSTED_ORIGINS=campus.aegisax.com \
  -e CONTACT_ADDRESS=security@example.com \
  aegiscampus
```

### 리버스 프록시 / 도메인 운영

리버스 프록시 뒤에서 도메인으로 접속하는 경우, CSRF 보호 때문에 **반드시 신뢰 출처에 도메인을 등록**해야 합니다. 등록하지 않으면 로그인·폼 제출이 차단됩니다.

- **바이너리 운영:** `config.json` 의 `admin_server.trusted_origins` 에 도메인 추가

  ```json
  "trusted_origins": ["campus.aegisax.com"]
  ```

- **Docker 운영:** `ADMIN_TRUSTED_ORIGINS=campus.aegisax.com` 환경변수로 주입

프록시는 admin 도메인을 컨테이너의 `:3333`, phishing 도메인을 `:8088` 로 전달하도록 구성합니다. TLS 종단을 프록시에서 처리하는 경우 `ADMIN_USE_TLS=false` 로 두고 프록시가 HTTPS 를 담당하게 할 수 있습니다.

---

## 릴리스 절차 / Release procedure

### 1. 버전 갱신

```
echo "1.0.0-rc8" > VERSION
```

[CHANGELOG.md](CHANGELOG.md) 에 해당 버전 섹션을 추가하고, `package.json` version 도 일치시킵니다 (`npm version <ver> --no-git-tag-version`).

### 2. 빌드 & 테스트

```
npx gulp build
go build -ldflags="-s -w" -trimpath -o aegiscampus .
go test ./...
```

### 3. 운영 배포

위 [운영 배포](#운영-배포--deployment-binary) 절차를 따릅니다.

### 4. Git 태그 + GitHub Release

```
git tag -a v1.0.0-rc8 -m "v1.0.0-rc8"
git push origin v1.0.0-rc8
```

GitHub 웹 UI 에서 `Releases > Draft a new release` → tag 선택 → CHANGELOG.md 의 해당 섹션을 release notes 로 작성 → `Publish release`. 릴리스 생성 시 `Build AegisCampus Release` 워크플로가 플랫폼별 바이너리 + 자산 묶음을 자동 빌드·첨부합니다.

---

## 문서 / Documentation

- [CHANGELOG.md](CHANGELOG.md) — 기능 인벤토리 및 릴리스 노트
- 소스 코드 `controllers/`, `models/`, `util/` — 동작 상세

## 이슈 / Issues

버그, 기능 요청, 문서 누락 모두 환영합니다. [이슈 등록](https://github.com/AegisAX/AegisCampus/issues).

---

<p align="center">
  <sub>AegisCampus is distributed under the MIT License. See <a href="LICENSE">LICENSE</a> for details.</sub>
</p>