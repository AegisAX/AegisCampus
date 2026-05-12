# 기여 가이드 / Contributing to Sentinel

Sentinel 에 기여해주셔서 감사합니다. 이 문서는 효율적인 기여를 위한 안내입니다.

*Thank you for your interest in contributing to Sentinel. This document outlines how to contribute effectively.*

---

## 이슈 신고 / Reporting issues

버그를 발견하거나 기능을 제안하고 싶으시면 [이슈 등록](https://github.com/AegisAX/Sentinel/issues) 을 통해 알려주세요. 이슈 템플릿이 필요한 정보를 안내해드립니다.

*To report a bug or suggest a feature, please [open an issue](https://github.com/AegisAX/Sentinel/issues). Issue templates will guide you through the required information.*

⚠️ 보안 취약점은 **공개 이슈로 신고하지 마세요**. [SECURITY.md](SECURITY.md) 의 비공개 신고 채널을 이용해주세요.

*Do not report security vulnerabilities via public issues. Use the private channel described in [SECURITY.md](SECURITY.md).*

---

## 기여 절차 / Contribution workflow

### 1. Fork & Branch

저장소를 fork 한 후 작업용 브랜치를 만듭니다. 브랜치 명명 규칙:

*Fork the repository and create a working branch. Naming convention:*

- `feat/<short-desc>` — 신규 기능 / new feature
- `fix/<short-desc>` — 버그 수정 / bug fix
- `chore/<short-desc>` — 빌드/구성 변경 / build or tooling
- `docs/<short-desc>` — 문서 변경 / documentation
- `refactor/<short-desc>` — 리팩터링 / refactoring

### 2. 개발 / Development

변경 사항 구현 후 로컬에서 테스트.

*Implement your changes and test locally.*

### 3. 빌드 검증 / Build verification

```
# Frontend assets
npx gulp build

# Backend
go build -ldflags="-s -w" -trimpath .
go test ./...
```

빌드와 테스트가 모두 통과해야 PR 가능합니다.

*All builds and tests must pass before a PR.*

### 4. 커밋 메시지 / Commit messages

간결한 한국어 또는 영문, conventional commit 스타일 권장.

*Concise Korean or English, conventional commit style preferred.*

예 / Examples:
- `fix(api): handle nil rid in /report-form`
- `feat(video): 영상 진행률 저장 주기 단축`
- `chore(deps): bump go from 1.23 to 1.24`

### 5. Pull Request

변경 의도, 테스트 결과, 관련 이슈 번호를 PR 설명에 포함해주세요.

*Include the intent, test results, and related issue numbers in the PR description.*

---

## 코드 스타일 / Code style

| 영역 / Area | 요구 사항 / Requirement |
|---|---|
| **Go** | `gofmt` + `goimports` 정렬, `go vet` 통과 |
| **JavaScript** | 변경 후 `npx gulp scripts` 로 minified 파일 동기화 |
| **CSS** | 변경 후 `npx gulp styles` 로 dist 동기화 |
| **Imports** | 미사용 import 는 commit 전 제거 (Go 컴파일러가 실패시킴) |
| **언어 / Language** | UI 사용자 노출 메시지는 한국어, 코드 주석은 영문 또는 한국어 자유 |

빌드 파이프라인 메모: JS / CSS 소스 변경 후 gulp 빌드를 하지 않으면 `dist/` 가 갱신되지 않아 변경이 적용되지 않은 것처럼 보입니다.

*Build pipeline note: JS / CSS source changes have no effect until gulp rebuilds the `dist/` files.*

---

## 리뷰 기준 / Review criteria

모든 PR 은 메인테이너 검토 후 머지됩니다. 검토 항목:

*All PRs are reviewed by maintainers before merging. Review criteria:*

- 기능 동작 / Functionality
- 보안 영향 (특히 인증/권한/입력 검증) / Security impact (auth, RBAC, input validation)
- 빌드 + 테스트 통과 / Build & test pass
- 문서 갱신 (필요 시) / Documentation updates if applicable
- 한국 기업 환경 호환성 (메일 인코딩, 한국어 처리) — 관련 변경 시
- *Korean enterprise compatibility (mail encoding, Korean handling) — when applicable*

---

## 질문 / Questions

기타 문의는 [GitHub Issues](https://github.com/AegisAX/Sentinel/issues) 를 이용해주세요.

*For other inquiries, please use GitHub Issues.*
