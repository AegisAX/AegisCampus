#!/usr/bin/env bash
#
# DOCKER-USER 허용목록 적용 — 정본은 deploy/docker-user.rules
#
# 기본은 dry-run 이다. 실제로 반영하려면 --apply 를 명시해야 한다.
# 방화벽 DROP 은 잘못 걸면 운영 서비스가 즉시 끊기므로, 계획을 눈으로 본 뒤 적용한다.
#
#   ./deploy/apply-docker-user.sh --npmplus-ip 10.0.0.5              # 계획만 출력
#   ./deploy/apply-docker-user.sh --npmplus-ip 10.0.0.5 --apply      # 실제 반영(root)
#   ./deploy/apply-docker-user.sh --revert --apply                   # 이 스크립트가 넣은 규칙 제거
#
# 이 스크립트가 넣은 규칙은 comment 태그로 식별한다. 재실행하면 기존 태그 규칙을
# 먼저 지우고 다시 넣으므로 멱등하다. 태그 없는 수기 규칙은 건드리지 않는다.

set -euo pipefail

TAG="aegiscampus-docker-user"
CHAIN="DOCKER-USER"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RULES_FILE="${SCRIPT_DIR}/docker-user.rules"

NPMPLUS_IP="${NPMPLUS_IP:-}"
WAN_IF="${WAN_IF:-}"
APPLY=0
REVERT=0

die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

usage() {
  sed -n '3,15p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  cat <<'EOF'

옵션:
  --npmplus-ip <IP|CIDR>  NPMplus 호스트 주소. 환경변수 NPMPLUS_IP 로도 지정 가능
  --wan-if <iface>        유입 인터페이스. 미지정 시 기본 라우트 인터페이스 자동 탐지
  --rules <path>          규칙 정본 경로 (기본: 스크립트와 같은 디렉토리의 docker-user.rules)
  --apply                 실제로 iptables 에 반영 (미지정 시 dry-run)
  --revert                태그된 규칙을 제거만 한다
  -h, --help              이 도움말
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --npmplus-ip) NPMPLUS_IP="${2:-}"; shift 2 ;;
    --wan-if)     WAN_IF="${2:-}";     shift 2 ;;
    --rules)      RULES_FILE="${2:-}"; shift 2 ;;
    --apply)      APPLY=1;  shift ;;
    --revert)     REVERT=1; shift ;;
    -h|--help)    usage; exit 0 ;;
    *) die "알 수 없는 인자: $1 (--help 참조)" ;;
  esac
done

command -v iptables >/dev/null 2>&1 || die "iptables 를 찾을 수 없다."

# --- 인터페이스 결정 ---------------------------------------------------------
if [ -z "$WAN_IF" ]; then
  WAN_IF="$(ip -4 route show default 2>/dev/null | awk '/ dev /{for(i=1;i<=NF;i++) if($i=="dev") {print $(i+1); exit}}')"
  [ -n "$WAN_IF" ] || die "기본 라우트 인터페이스를 자동 탐지하지 못했다. --wan-if 로 지정할 것."
  printf '자동 탐지한 유입 인터페이스: %s (다르면 --wan-if 로 지정)\n' "$WAN_IF"
fi
ip link show "$WAN_IF" >/dev/null 2>&1 || die "인터페이스가 존재하지 않는다: $WAN_IF"

# --- 규칙 스펙 구성 ----------------------------------------------------------
declare -a SPECS=()
if [ "$REVERT" -eq 0 ]; then
  [ -f "$RULES_FILE" ] || die "규칙 정본이 없다: $RULES_FILE"
  [ -n "$NPMPLUS_IP" ] || die "NPMplus 호스트 IP 가 필요하다 (--npmplus-ip 또는 NPMPLUS_IP)."
  # 단일 IPv4 또는 CIDR 만 받는다. 오타로 엉뚱한 대역을 여는 사고를 막는다.
  if ! printf '%s' "$NPMPLUS_IP" | grep -Eq '^([0-9]{1,3}\.){3}[0-9]{1,3}(/([0-9]|[12][0-9]|3[0-2]))?$'; then
    die "NPMplus 주소 형식이 잘못됐다(IPv4 또는 CIDR 이어야 함): $NPMPLUS_IP"
  fi

  while IFS= read -r line; do
    line="${line%%#*}"
    line="$(printf '%s' "$line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
    [ -n "$line" ] || continue
    line="${line//%NPMPLUS_IP%/$NPMPLUS_IP}"
    line="${line//%WAN_IF%/$WAN_IF}"
    case "$line" in
      *%*) die "치환되지 않은 토큰이 남았다: $line" ;;
    esac
    SPECS+=("$line -m comment --comment $TAG")
  done < "$RULES_FILE"

  [ "${#SPECS[@]}" -gt 0 ] || die "규칙 정본에서 읽어낸 규칙이 0개다: $RULES_FILE"
fi

# --- 계획 출력 ---------------------------------------------------------------
echo
echo "=== 계획 ==="
if [ "$REVERT" -eq 1 ]; then
  echo "태그 '$TAG' 가 붙은 $CHAIN 규칙을 전부 제거한다."
else
  echo "1) 태그 '$TAG' 가 붙은 기존 $CHAIN 규칙 제거 (멱등성 확보)"
  echo "2) 아래 규칙을 $CHAIN 맨 앞에 기재 순서대로 삽입:"
  i=1
  for spec in "${SPECS[@]}"; do
    printf '     iptables -I %s %d %s\n' "$CHAIN" "$i" "$spec"
    i=$((i + 1))
  done
fi
echo

if [ "$APPLY" -eq 0 ]; then
  cat <<EOF
=== dry-run 이다. iptables 를 건드리지 않았다. ===
실제로 반영하려면 위 계획을 확인한 뒤 root 로 --apply 를 붙여 재실행할 것:

  sudo $0 $([ -n "$NPMPLUS_IP" ] && printf -- '--npmplus-ip %s ' "$NPMPLUS_IP")--wan-if $WAN_IF $([ "$REVERT" -eq 1 ] && printf -- '--revert ')--apply
EOF
  exit 0
fi

# --- 실제 반영 ---------------------------------------------------------------
[ "$(id -u)" -eq 0 ] || die "--apply 는 root 권한이 필요하다 (sudo)."
iptables -S "$CHAIN" >/dev/null 2>&1 \
  || die "$CHAIN 체인이 없다. docker 가 실행 중인지 확인할 것(체인은 docker 가 만든다)."

# 기존 태그 규칙 제거: -A 를 -D 로 바꿔 되돌린다. 없으면 아무것도 하지 않는다.
removed=0
while IFS= read -r arule; do
  [ -n "$arule" ] || continue
  # shellcheck disable=SC2086
  iptables ${arule/#-A/-D}
  removed=$((removed + 1))
done < <(iptables -S "$CHAIN" | grep -F -- "--comment \"$TAG\"" || true)
printf '기존 태그 규칙 제거: %d 건\n' "$removed"

if [ "$REVERT" -eq 0 ]; then
  i=1
  for spec in "${SPECS[@]}"; do
    # shellcheck disable=SC2086
    iptables -I "$CHAIN" "$i" $spec
    i=$((i + 1))
  done
  printf '규칙 삽입: %d 건\n' "${#SPECS[@]}"
fi

echo
echo "=== 반영 후 $CHAIN 현재 상태 ==="
iptables -S "$CHAIN"

if [ "$REVERT" -eq 0 ]; then
  cat <<EOF

=== 남은 일: 규칙이 실제로 먹는지 실측할 것 ("규칙 있음"은 증거가 아니다) ===
  1) 차단 확인  — NPMplus 가 아닌 LAN 호스트에서:
       curl -k --connect-timeout 5 https://<이 호스트 IP>:3333/   # 타임아웃/거부여야 정상
       curl    --connect-timeout 5 http://<이 호스트 IP>:8088/     # 타임아웃/거부여야 정상
  2) 과차단 아님 확인 — 대조군:
       campus.aegisax.com / campaign.aegisax.com 을 NPMplus 경유로 열어 정상 응답 확인
  3) 둘 다 확인된 뒤에야 CLAUDE.md 를 "적용 완료"로 고칠 것.

주의: 이 규칙은 호스트 재부팅·iptables 플러시로 사라진다. 영속화가 필요하면
      부팅 시 이 스크립트를 --apply 로 재실행하도록 걸어둘 것.
EOF
fi
