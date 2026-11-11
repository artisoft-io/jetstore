#!/usr/bin/env bash
# Takes down the stack up.sh started for this checkout: the apiserver and its
# bridges, the database container, and the temporary directory. Safe to run
# when nothing is running, and run by `npm run shots` whether the specs passed
# or not.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

if [ -f "$STATE_FILE" ]; then
  # shellcheck disable=SC1090
  source "$STATE_FILE"
fi

stop_pid() { # stop_pid <pid> [group]
  local pid="$1" target="$1"
  [ -n "$pid" ] || return 0
  [ "${2:-}" = group ] && target="-$pid"
  kill -TERM -- "$target" 2> /dev/null || return 0
  for _ in $(seq 1 20); do
    kill -0 -- "$target" 2> /dev/null || return 0
    sleep 0.25
  done
  kill -KILL -- "$target" 2> /dev/null || true
}

stop_pid "${APISERVER_PGID:-}" group
stop_pid "${HTTP_BRIDGE_PID:-}"
stop_pid "${PG_BRIDGE_PID:-}"

if docker container inspect "$STACK_NAME" > /dev/null 2>&1; then
  log "removing container $STACK_NAME"
  docker rm -f "$STACK_NAME" > /dev/null
fi

rm -rf "$RUN_DIR"
rm -f "$STATE_FILE"
exit 0
