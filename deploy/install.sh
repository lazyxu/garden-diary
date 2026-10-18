#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

REPOSITORY="lazyxu/garden-diary"
ROOT="${GD_HOME:-$HOME/.garden-diary}"
CONFIG="$ROOT/config"
BIN="$ROOT/bin"
STATE="$ROOT/state"
DATA_ROOT="$ROOT/data"
ENV="$CONFIG/.env"
COMPOSE="$CONFIG/docker-compose.yml"
BACKUP_ROOT="$ROOT/backups/pre-upgrade"
CHANNEL=""
REF=""
TAG=""
TEMP=""
PREVIOUS=0
ROLLBACK_ARMED=0
SNAPSHOT=""
SNAPSHOT_VALID=0
PRESERVE_TEMP=0
STAGE="startup"

log() { printf '\n[花园日记] %s\n' "$*"; }
error() { printf '[花园日记] 错误（阶段 %s）：%s\n' "$STAGE" "$*" >&2; }

require_command() { command -v "$1" >/dev/null 2>&1 || { error "缺少必要命令：$1"; exit 1; }; }
getenv() {
  [[ -f "$ENV" ]] || return 0
  grep -m1 "^$1=" "$ENV" 2>/dev/null | cut -d= -f2- || true
}
setenv() {
  local key="$1" value="$2" next
  next="$(mktemp "$CONFIG/.env.XXXXXX")"
  awk -v key="$key" 'index($0,key"=")!=1{print}' "$ENV" > "$next"
  printf '%s=%s\n' "$key" "$value" >> "$next"
  chmod 600 "$next"
  mv -f "$next" "$ENV"
}
defaultenv() {
  [[ -n "$(getenv "$1")" ]] || setenv "$1" "$2"
}
compose() { docker compose --env-file "$ENV" -f "$COMPOSE" "$@"; }
safe_data_dir() {
  local path="$1"
  [[ "$path" == /* && "$path" != "/" && "$path" != "$HOME" && "$path" != "$ROOT" ]] || {
    error "危险或无效的数据目录：$path";exit 1;
  }
}
restore_snapshot() {
  local data="$1" archive="$2"
  safe_data_dir "$data"
  [[ -f "$archive" ]] || return 1
  tar -tzf "$archive" >/dev/null
  find "$data" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
  tar -C "$data" -xzf "$archive" --no-same-owner --no-same-permissions
}
rollback() {
  [[ "$PREVIOUS" -eq 1 ]] || return 0
  log "更新未成功，恢复原版本与数据"
  if [[ -f "$COMPOSE" && -f "$ENV" ]]; then compose --profile https stop app web web-https >/dev/null 2>&1 || true; fi
  cp -f "$TEMP/old.env" "$ENV"
  cp -f "$TEMP/old-compose.yml" "$COMPOSE"
  if [[ -f "$TEMP/old-Caddyfile.http" ]]; then cp -f "$TEMP/old-Caddyfile.http" "$CONFIG/Caddyfile.http"; fi
  if [[ -f "$TEMP/old-Caddyfile.https" ]]; then cp -f "$TEMP/old-Caddyfile.https" "$CONFIG/Caddyfile.https"; fi
  chmod 600 "$ENV"
  local old_data
  old_data="$(getenv GD_DATA_DIR)"
  if [[ "$SNAPSHOT_VALID" -eq 1 && -n "$SNAPSHOT" && -f "$SNAPSHOT" ]]; then restore_snapshot "$old_data" "$SNAPSHOT" || { error "数据快照还原失败，请立即人工处理：$SNAPSHOT";return 1; }; fi
  compose up -d --force-recreate app web
  if [[ -n "$(getenv GD_DOMAIN)" ]]; then compose --profile https up -d --force-recreate web-https; fi
  log "已尝试恢复旧容器和数据；升级快照保留在 $SNAPSHOT"
}
on_exit() {
  local rc="$?"
  trap - EXIT ERR
  if [[ "$rc" -ne 0 && "$ROLLBACK_ARMED" -eq 1 ]]; then
    if ! rollback; then PRESERVE_TEMP=1;error "回滚未完成，旧配置与快照位于 $TEMP 和 $SNAPSHOT"; rc=70; fi
  fi
  if [[ "$PRESERVE_TEMP" -eq 0 && -n "$TEMP" ]]; then rm -rf "$TEMP"; fi
  exit "$rc"
}
trap on_exit EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

while (( $# )); do
  case "$1" in
    --channel) CHANNEL="${2:-}";shift 2;;
    --help|-h)
      echo "用法: bash deploy/install.sh [--channel master|stable]"
      exit 0;;
    *) error "未知参数：$1";exit 2;;
  esac
done

STAGE="prerequisites"
for cmd in curl docker tar awk flock mktemp; do require_command "$cmd"; done
docker info >/dev/null
docker compose version >/dev/null

mkdir -p "$ROOT" "$CONFIG" "$BIN" "$STATE" "$DATA_ROOT/app" "$DATA_ROOT/caddy/data" "$DATA_ROOT/caddy/config" "$BACKUP_ROOT"
chmod 700 "$ROOT" "$CONFIG" "$BIN" "$STATE" "$DATA_ROOT" "$BACKUP_ROOT"
exec 9>"$STATE/install.lock"
flock -n 9 || { error "已经有一个安装或升级任务正在运行";exit 1; }
TEMP="$(mktemp -d "$STATE/install.XXXXXX")"

[[ -n "$CHANNEL" ]] || CHANNEL="$(getenv GD_CHANNEL)"
CHANNEL="${CHANNEL:-master}"
[[ "$CHANNEL" == "master" || "$CHANNEL" == "stable" ]] || { error "仅支持 master 或 stable 渠道"; exit 2; }

STAGE="resolve-release"
if [[ "$CHANNEL" == "master" ]]; then
  response="$(curl --fail --silent --show-error --location --retry 3 --connect-timeout 15 "https://api.github.com/repos/$REPOSITORY/commits/master")"
  REF="$(printf '%s\n' "$response" | sed -nE 's/^[[:space:]]*"sha":[[:space:]]*"([a-f0-9]{40})".*/\1/p' | head -n1)"
  [[ "$REF" =~ ^[a-f0-9]{40}$ ]] || { error "无法解析 master 的完整 commit SHA"; exit 1; }
  TAG="sha-${REF:0:12}"
else
  response="$(curl --fail --silent --show-error --location --retry 3 --connect-timeout 15 "https://api.github.com/repos/$REPOSITORY/releases/latest")"
  REF="$(printf '%s\n' "$response" | sed -nE 's/^[[:space:]]*"tag_name":[[:space:]]*"([^"]+)".*/\1/p' | head -n1)"
  [[ "$REF" =~ ^v[0-9]+[.][0-9]+[.][0-9]+([-+._a-zA-Z0-9]*)?$ ]] || { error "没有找到有效的稳定版发布"; exit 1; }
  TAG="$REF"
fi
log "目标版本：$CHANNEL / $REF"

STAGE="stage-files"
base="https://raw.githubusercontent.com/$REPOSITORY/$REF"
download() { curl --fail --silent --show-error --location --retry 4 --retry-delay 2 --connect-timeout 15 "$base/$1" -o "$TEMP/$2"; }
download deploy/docker-compose.yml docker-compose.yml
download deploy/Caddyfile.http Caddyfile.http
download deploy/Caddyfile.https Caddyfile.https
download scripts/garden-diary garden-diary
download deploy/install.sh install.sh
bash -n "$TEMP/garden-diary"
bash -n "$TEMP/install.sh"

PREVIOUS=0
if [[ -f "$ENV" && -f "$COMPOSE" ]]; then
  PREVIOUS=1
  cp -p "$ENV" "$TEMP/old.env"
  cp -p "$COMPOSE" "$TEMP/old-compose.yml"
  ROLLBACK_ARMED=1
  for file in Caddyfile.http Caddyfile.https; do
    [[ ! -f "$CONFIG/$file" ]] || cp -p "$CONFIG/$file" "$TEMP/old-$file"
  done
fi

STAGE="config"
touch "$ENV"
chmod 600 "$ENV"
random_secret() {
  if command -v openssl >/dev/null; then openssl rand -hex 32
  else head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'; fi
}
defaultenv GD_SETUP_TOKEN "$(random_secret)"
defaultenv GD_UID "$(id -u)"
defaultenv GD_GID "$(id -g)"
defaultenv GD_DATA_DIR "$DATA_ROOT/app"
defaultenv GD_CADDY_DATA_DIR "$DATA_ROOT/caddy/data"
defaultenv GD_CADDY_CONFIG_DIR "$DATA_ROOT/caddy/config"
defaultenv GD_HTTP_BIND "0.0.0.0"
defaultenv GD_HTTP_PORT "3018"
defaultenv GD_HTTPS_HTTP_PORT "80"
defaultenv GD_HTTPS_PORT "443"
safe_data_dir "$(getenv GD_DATA_DIR)"
mkdir -p "$(getenv GD_DATA_DIR)" "$(getenv GD_CADDY_DATA_DIR)" "$(getenv GD_CADDY_CONFIG_DIR)"
setenv GD_CHANNEL "$CHANNEL"
setenv GD_REF "$REF"
setenv GD_APP_IMAGE "ghcr.io/lazyxu/garden-diary-api:$TAG"
setenv GD_WEB_IMAGE "ghcr.io/lazyxu/garden-diary-web:$TAG"

STAGE="snapshot"
if [[ "$PREVIOUS" -eq 1 ]]; then
  # Stop SQLite writer before copying the complete DB, WAL and uploaded originals.
  compose_old() { docker compose --env-file "$TEMP/old.env" -f "$TEMP/old-compose.yml" "$@"; }
  # Old compose may use relative paths, so run through the original file path before replacing it.
  docker compose --env-file "$TEMP/old.env" -f "$COMPOSE" stop app >/dev/null 2>&1 || true
  SNAPSHOT="$BACKUP_ROOT/garden-pre-upgrade-$(date -u +%Y%m%dT%H%M%SZ)-$$.tar.gz"
  tar -C "$(getenv GD_DATA_DIR)" -czf "$SNAPSHOT" .
  tar -tzf "$SNAPSHOT" >/dev/null
  SNAPSHOT_VALID=1
  log "升级前备份：$SNAPSHOT"
fi

STAGE="apply-files"
cp -f "$TEMP/docker-compose.yml" "$COMPOSE"
cp -f "$TEMP/Caddyfile.http" "$CONFIG/Caddyfile.http"
cp -f "$TEMP/Caddyfile.https" "$CONFIG/Caddyfile.https"
docker compose --env-file "$ENV" -f "$COMPOSE" config --quiet

STAGE="pull-images"
compose pull app
compose pull web
if [[ -n "$(getenv GD_DOMAIN)" ]]; then compose --profile https pull web-https; fi

STAGE="start"
compose up -d --force-recreate app web
if [[ -n "$(getenv GD_DOMAIN)" ]]; then compose --profile https up -d --force-recreate web-https; fi

STAGE="healthcheck"
healthy() { [[ "$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$1" 2>/dev/null || true)" == "healthy" ]]; }
ok=0
for i in {1..50}; do
  if healthy garden-diary-app && healthy garden-diary-web; then ok=1;break;fi
  sleep 2
done
[[ "$ok" -eq 1 ]] || {compose ps || true;error "新版本未通过健康检查"; exit 1; }

STAGE="complete"
cp -f "$TEMP/garden-diary" "$BIN/garden-diary"
cp -f "$TEMP/install.sh" "$BIN/install.sh"
chmod 700 "$BIN/garden-diary" "$BIN/install.sh"
ROLLBACK_ARMED=0
log "成功安装/更新至 $REF"
log "局域网访问：http://服务器IP:$(getenv GD_HTTP_PORT)"
if [[ "$PREVIOUS" -eq 0 ]]; then
  printf '\n首次初始化管理员时使用的口令（请妥善保管）：\n%s\n' "$(getenv GD_SETUP_TOKEN)"
fi
log "管理命令：$BIN/garden-diary status | update | backup | restore | doctor"
