#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY_NAME="codex-register"

killed=0

# 1) Kill any running compiled binary named codex-register
pids=$(pgrep -x "$BINARY_NAME" 2>/dev/null || true)
if [[ -n "$pids" ]]; then
    echo "Stopping running $BINARY_NAME (pid: $pids) ..."
    echo "$pids" | xargs kill 2>/dev/null || true
    killed=1
fi

# 2) Kill any "go run" process launched from this project directory
go_run_pids=$(pgrep -f "go run.*${PROJECT_DIR}" 2>/dev/null || true)
if [[ -n "$go_run_pids" ]]; then
    echo "Stopping go run process (pid: $go_run_pids) ..."
    echo "$go_run_pids" | xargs kill 2>/dev/null || true
    killed=1
fi

# 3) Also catch the compiled temp binary that "go run" spawns (path contains module name)
tmp_bin_pids=$(pgrep -f "/exe/${BINARY_NAME}" 2>/dev/null || true)
if [[ -n "$tmp_bin_pids" ]]; then
    echo "Stopping temp binary (pid: $tmp_bin_pids) ..."
    echo "$tmp_bin_pids" | xargs kill 2>/dev/null || true
    killed=1
fi

if [[ $killed -eq 1 ]]; then
    sleep 1
    echo "Previous process(es) terminated."
fi

echo "Starting $BINARY_NAME in $PROJECT_DIR ..."
cd "$PROJECT_DIR"
exec go run .
