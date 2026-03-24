#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/gpt-register}"
SRC_DIR="${SRC_DIR:-$APP_DIR/src}"
BIN_DIR="${BIN_DIR:-$APP_DIR/bin}"
BIN_NAME="${BIN_NAME:-codex-register}"
SERVICE_NAME="${SERVICE_NAME:-gpt-register}"
REPO_URL="${REPO_URL:-https://github.com/wgka/gpt-register.git}"
BRANCH="${BRANCH:-master}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:18180/}"
KEEP_BACKUPS="${KEEP_BACKUPS:-10}"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"

log() {
  printf '[deploy] %s\n' "$*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

if [[ "${EUID}" -ne 0 ]]; then
  echo "please run as root" >&2
  exit 1
fi

require_cmd git
require_cmd go
require_cmd node
require_cmd npm
require_cmd systemctl
require_cmd curl

mkdir -p "$APP_DIR" "$BIN_DIR"

if [[ ! -f "$APP_DIR/.env" ]]; then
  echo "missing env file: $APP_DIR/.env" >&2
  exit 1
fi

if [[ ! -d "$SRC_DIR/.git" ]]; then
  log "cloning $REPO_URL -> $SRC_DIR"
  rm -rf "$SRC_DIR"
  git clone --branch "$BRANCH" --single-branch "$REPO_URL" "$SRC_DIR"
else
  log "updating source in $SRC_DIR"
  git -C "$SRC_DIR" fetch origin "$BRANCH"
  git -C "$SRC_DIR" checkout "$BRANCH"
  git -C "$SRC_DIR" reset --hard "origin/$BRANCH"
  git -C "$SRC_DIR" clean -fd
fi

COMMIT="$(git -C "$SRC_DIR" rev-parse --short HEAD)"
log "deploying commit $COMMIT"

log "building frontend"
(
  cd "$SRC_DIR/frontend"
  npm ci
  npm run build
)

log "building backend"
(
  cd "$SRC_DIR"
  go build -o "$BIN_DIR/$BIN_NAME.new" .
)
chmod 755 "$BIN_DIR/$BIN_NAME.new"

if [[ -f "$BIN_DIR/$BIN_NAME" ]]; then
  BACKUP_PATH="$BIN_DIR/$BIN_NAME.pre-$TIMESTAMP.backup"
  log "backing up current binary -> $BACKUP_PATH"
  cp "$BIN_DIR/$BIN_NAME" "$BACKUP_PATH"
fi

log "replacing binary"
mv "$BIN_DIR/$BIN_NAME.new" "$BIN_DIR/$BIN_NAME"

log "restarting service $SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

for _ in $(seq 1 20); do
  if curl -fsS "$HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

log "service status"
systemctl --no-pager --full status "$SERVICE_NAME" | sed -n '1,20p'

log "health check"
curl -fsS "$HEALTH_URL" >/dev/null
log "health OK: $HEALTH_URL"

if [[ "$KEEP_BACKUPS" =~ ^[0-9]+$ ]] && (( KEEP_BACKUPS > 0 )); then
  mapfile -t backups < <(find "$BIN_DIR" -maxdepth 1 -type f -name "$BIN_NAME.pre-*.backup" | sort -r)
  if (( ${#backups[@]} > KEEP_BACKUPS )); then
    for old_backup in "${backups[@]:KEEP_BACKUPS}"; do
      rm -f "$old_backup"
    done
  fi
fi

log "done"
