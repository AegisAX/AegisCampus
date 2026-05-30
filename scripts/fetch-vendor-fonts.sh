#!/usr/bin/env bash
# scripts/fetch-vendor-fonts.sh
#
# AegisCampus — 외부 Google Fonts CDN 의존 제거를 위한 WOFF2 다운로드.
# 1회 실행하여 static/font/ 에 폰트 파일을 받는다. 이후 오프라인 빌드/배포 가능.
#
# 소스:
#   - Source Sans 3 (Source Sans Pro 후계, metric 호환)
#     Adobe 공식 GitHub release / SIL OFL 1.1
#     Light(300) / Regular(400) / Semibold(600) / Bold(700) — 전체 글리프
#   - Roboto
#     Google Fonts CDN 정적 WOFF2 (v30, latin 서브셋) / Apache 2.0
#     Medium(500) / Bold(700) — 영문 헤더 전용, 한글은 시스템 폰트 fallback

set -euo pipefail

cd "$(dirname "$0")/.."
DEST="static/font"
mkdir -p "$DEST"

# Adobe Source Sans 3 — Adobe 공식 release/WOFF2/OTF
SS_BASE="https://raw.githubusercontent.com/adobe-fonts/source-sans/release/WOFF2/OTF"
declare -A SOURCE_SANS=(
  ["SourceSans3-Light.otf.woff2"]="sourcesans3-light.woff2"
  ["SourceSans3-Regular.otf.woff2"]="sourcesans3-regular.woff2"
  ["SourceSans3-Semibold.otf.woff2"]="sourcesans3-semibold.woff2"
  ["SourceSans3-Bold.otf.woff2"]="sourcesans3-bold.woff2"
)

# Roboto v30 latin 서브셋 (Google Fonts CDN 정적 호스팅, 해시 기반 영구 URL)
RB_BASE="https://fonts.gstatic.com/s/roboto/v30"
declare -A ROBOTO=(
  ["KFOlCnqEu92Fr1MmEU9fBBc4.woff2"]="roboto-medium.woff2"
  ["KFOlCnqEu92Fr1MmWUlfBBc4.woff2"]="roboto-bold.woff2"
)

download() {
  local url="$1"
  local out="$2"
  if [[ -f "$out" ]]; then
    echo "  skip (exists): $out"
    return
  fi
  echo "  fetching: $url"
  curl -fsSL -A "Mozilla/5.0" --retry 3 --retry-delay 2 -o "$out" "$url"
  if [[ ! -s "$out" ]]; then
    echo "  ERROR: empty file $out" >&2
    rm -f "$out"
    exit 1
  fi
}

echo "[1/2] Source Sans 3 (SIL OFL 1.1)"
for src in "${!SOURCE_SANS[@]}"; do
  download "$SS_BASE/$src" "$DEST/${SOURCE_SANS[$src]}"
done

echo "[2/2] Roboto (Apache 2.0)"
for src in "${!ROBOTO[@]}"; do
  download "$RB_BASE/$src" "$DEST/${ROBOTO[$src]}"
done

echo "[license] Source Sans 3 / Roboto"
download "https://raw.githubusercontent.com/adobe-fonts/source-sans/release/LICENSE.md" \
         "$DEST/LICENSE-SourceSans.md"
download "https://raw.githubusercontent.com/google/roboto/master/LICENSE" \
         "$DEST/LICENSE-Roboto.txt"

echo ""
echo "Done. Files in $DEST/:"
ls -la "$DEST"/sourcesans3-*.woff2 "$DEST"/roboto-*.woff2 \
       "$DEST"/LICENSE-* 2>/dev/null || true
