#!/usr/bin/env bash
# Starts a local JetStore for the screenshot specs, and seeds it.
#
#   JETS_BIN_DIR=/path/to/bin JETS_ADMIN_PWD=... docs/screenshots/stack/up.sh
#
# `npm run shots` runs this, then the specs, then down.sh. Run it by hand only
# to look at the stack yourself; then run down.sh when you are done.
#
# In order:
#   1. the binaries in JETS_BIN_DIR, built by build-bin.sh when it is empty;
#   2. jetsclient_ide/dist and docs/dist, rebuilt when their sources are newer;
#   3. jets_ws exported with `git archive` into a temporary directory, its
#      JetStore assets installed, and the workspace compiled;
#   4. postgres:16-alpine on a free port, in a container named for this checkout;
#   5. update_db: the system tables, the base configuration, and jets_ws's CI
#      client configuration (process_config/ci_workspace_init_db.sql), loaded as
#      an installation loads a client's configuration;
#   6. every user_email set to the stack's admin, then the fixtures in
#      docs/screenshots/fixtures/, in name order;
#   7. the apiserver, git off and in dev mode, in a network namespace of its own,
#      reached on a free port of 127.0.0.1.
#
# It prints the stack's address on its last line, as STACK_URL=http://127.0.0.1:<port>.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

: "${JETS_BIN_DIR:?set JETS_BIN_DIR to the directory of the JetStore binaries (see build-bin.sh)}"
: "${JETS_ADMIN_PWD:?set JETS_ADMIN_PWD to the admin password for this run}"
case "$JETS_BIN_DIR" in
  /usr/local/bin|/usr/local/bin/) die "JETS_BIN_DIR must not be /usr/local/bin" ;;
esac

for tool in docker socat unshare ip git node npm python3; do
  command -v "$tool" > /dev/null || die "$tool is not installed"
done
unshare -rn true 2> /dev/null || die "this machine does not allow 'unshare -rn' (user and network namespaces)"

# A stack left by an interrupted run is taken down first.
if [ -f "$STATE_FILE" ] || docker container inspect "$STACK_NAME" > /dev/null 2>&1; then
  log "taking down the stack a previous run left"
  "$STACK_DIR/down.sh"
fi
mkdir -p "$STATE_DIR"
: > "$STATE_FILE"
chmod 600 "$STATE_FILE"
record() { printf '%s=%q\n' "$1" "$2" >> "$STATE_FILE"; }
trap 'log "start failed; taking the stack down"; "$STACK_DIR/down.sh"' ERR

# 1. Binaries.
if [ ! -x "$JETS_BIN_DIR/apiserver" ]; then
  log "no binaries in $JETS_BIN_DIR; building them"
  "$STACK_DIR/build-bin.sh"
fi
BIN="$(cd "$JETS_BIN_DIR" && pwd)"
log "binaries: $BIN ($(tr '\n' ' ' < "$BIN/BUILD_INFO" 2> /dev/null || echo 'no BUILD_INFO'))"

# 2. The web app and the documentation site, from this checkout.
stale() { # stale <dist marker> <source dir>...
  local marker="$1"; shift
  [ ! -f "$marker" ] && return 0
  [ -n "$(find "$@" -newer "$marker" -type f -print -quit 2> /dev/null)" ]
}
IDE_DIR="$REPO_ROOT/jetsclient_ide"
if [ "${SHOTS_SKIP_WEB_BUILD:-}" != 1 ] && stale "$IDE_DIR/dist/index.html" "$IDE_DIR/src" "$IDE_DIR/public" "$IDE_DIR/package.json"; then
  log "building jetsclient_ide/dist"
  ( cd "$IDE_DIR" && { [ -d node_modules ] || npm ci --no-audit --no-fund; } && npx vite build --logLevel warn )
fi
DOCS_DIR="$REPO_ROOT/docs"
if [ "${SHOTS_SKIP_WEB_BUILD:-}" != 1 ] && stale "$DOCS_DIR/dist/index.html" "$DOCS_DIR/src" "$DOCS_DIR/public" "$DOCS_DIR/astro.config.mjs"; then
  log "building docs/dist"
  ( cd "$DOCS_DIR" && { [ -d node_modules ] || npm ci --no-audit --no-fund; } && npm run --silent build > "$STATE_DIR/docs-build.log" 2>&1 ) \
    || die "the documentation build failed; see $STATE_DIR/docs-build.log"
fi
[ -f "$IDE_DIR/dist/index.html" ] || die "jetsclient_ide/dist is missing"
[ -f "$DOCS_DIR/dist/index.html" ] || die "docs/dist is missing"

# 3. The workspace: an export, never a link, so nothing the stack does reaches the repository.
[ -d "$JETS_WS_REPO" ] || die "JETS_WS_REPO=$JETS_WS_REPO is not a directory; point it at a jets_ws repository"
rm -rf "$RUN_DIR"
mkdir -p "$RUN_DIR/workspaces/$STACK_WORKSPACE" "$RUN_DIR/sock"
record RUN_DIR "$RUN_DIR"
log "exporting jets_ws at $JETS_WS_REF"
git -C "$JETS_WS_REPO" archive --format=tar "$JETS_WS_REF" | tar -x -C "$RUN_DIR/workspaces/$STACK_WORKSPACE"

export WORKSPACES_HOME="$RUN_DIR/workspaces"
export WORKSPACE="$STACK_WORKSPACE"
export WORKSPACE_BRANCH="$STACK_WORKSPACE_BRANCH"
export JETS_VERSION="$STACK_JETS_VERSION"
export JETS_SCHEMA_FILE="$BIN/jets_schema.json"
export JETS_INIT_DB_SCRIPT="$BIN/jets_init_db.sql"
export JETS_WORKSPACE_DB_SCHEMA_SCRIPT="$BIN/workspace_schema.sql"
export JETS_WORKSPACE_VALIDATOR="$BIN/jets_validate_workspace.mjs"
export JETS_RULES_UNIT_TEST_HARNESS="$BIN/rules_unit_test"
export LD_LIBRARY_PATH="$BIN${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

log "installing JetStore's assets into the workspace and compiling it"
( cd "$RUN_DIR" && "$BIN/install_workspace_assets" -w="$WORKSPACE" > "$STATE_DIR/install_workspace_assets.log" 2>&1 ) \
  || die "install_workspace_assets failed; see $STATE_DIR/install_workspace_assets.log"
( cd "$RUN_DIR" && "$BIN/compile_workspace" -w="$WORKSPACE" -v="$JETS_VERSION" > "$STATE_DIR/compile_workspace.log" 2>&1 ) \
  || die "compile_workspace failed; see $STATE_DIR/compile_workspace.log"

# 4. The database.
free_port() { python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()'; }
PG_PORT="$(free_port)"
PG_PASSWORD="$(head -c 18 /dev/urandom | base64 | tr -d '/+=')"
log "starting postgres:16-alpine as $STACK_NAME on 127.0.0.1:$PG_PORT"
record STACK_NAME "$STACK_NAME"
docker run -d --rm --name "$STACK_NAME" \
  --label "jetstore.screenshots.checkout=$REPO_ROOT" \
  -e POSTGRES_PASSWORD="$PG_PASSWORD" \
  -p "127.0.0.1:$PG_PORT:5432" \
  postgres:16-alpine > /dev/null
record PG_PORT "$PG_PORT"
for _ in $(seq 1 60); do
  docker exec "$STACK_NAME" pg_isready -q -h 127.0.0.1 -U postgres 2> /dev/null && break
  sleep 1
done
docker exec "$STACK_NAME" pg_isready -q -h 127.0.0.1 -U postgres || die "postgres did not start"
export JETS_DSN_URI_VALUE="postgresql://postgres:$PG_PASSWORD@127.0.0.1:$PG_PORT/postgres?pool_max_conns=10"

# 5. The system tables and the configuration.
log "migrating the database and loading jets_ws's CI configuration"
( cd "$RUN_DIR" && "$BIN/update_db" -usingSshTunnel -migrateDb -initBaseWorkspaceDb -clients CI > "$STATE_DIR/update_db.log" 2>&1 ) \
  || die "update_db failed; see $STATE_DIR/update_db.log"
stack_psql -v version="$JETS_VERSION" <<'SQL'
INSERT INTO jetsapi.jetstore_release (version) VALUES (:'version');
SQL

# 6. No real address on any screen, then the fixtures.
rewrite_user_email() {
  stack_psql -v admin="$STACK_ADMIN_EMAIL" < "$STACK_DIR/user_email.sql"
}
rewrite_user_email
# The fixtures read the data files as /fixture-data/<file> in the container.
docker cp "$SHOTS_DIR/data/." "$STACK_NAME:/fixture-data" > /dev/null
for fixture in "$SHOTS_DIR"/fixtures/*.sql; do
  [ -e "$fixture" ] || continue
  log "applying fixtures/$(basename "$fixture")"
  stack_psql < "$fixture" || die "fixture $(basename "$fixture") failed"
done
rewrite_user_email

# 7. The apiserver, in its own network namespace.
HTTP_PORT="$(free_port)"
SOCK_DIR="$RUN_DIR/sock"
# Started detached from this script's output, so a caller reading it sees its end.
socat "UNIX-LISTEN:$SOCK_DIR/pg.sock,fork,unlink-early" "TCP:127.0.0.1:$PG_PORT" \
  >> "$STATE_DIR/bridges.log" 2>&1 < /dev/null &
record PG_BRIDGE_PID "$!"
socat "TCP-LISTEN:$HTTP_PORT,bind=127.0.0.1,fork,reuseaddr" "UNIX-CONNECT:$SOCK_DIR/http.sock" \
  >> "$STATE_DIR/bridges.log" 2>&1 < /dev/null &
record HTTP_BRIDGE_PID "$!"
record HTTP_PORT "$HTTP_PORT"

API_SECRET="$(head -c 32 /dev/urandom | base64)" \
JETS_ENCRYPTION_KEY="$(head -c 24 /dev/urandom | base64)" \
JETS_DSN_URI_VALUE="postgresql://postgres:$PG_PASSWORD@127.0.0.1:5432/postgres?pool_max_conns=10" \
JETS_NO_GIT_ACCESS=true \
JETSTORE_DEV_MODE=1 \
JETS_s3_INPUT_PREFIX=jetstore/input \
JETS_s3_OUTPUT_PREFIX=jetstore/output \
JETS_s3_STAGE_PREFIX=jetstore/stage \
SOCK_DIR="$SOCK_DIR" JETS_BIN_DIR="$BIN" STACK_ADMIN_EMAIL="$STACK_ADMIN_EMAIL" \
IDE_DIST="$IDE_DIR/dist" DOCS_DIST="$DOCS_DIR/dist" \
  setsid unshare -rn --kill-child bash "$STACK_DIR/apiserver-netns.sh" \
  > "$STATE_DIR/apiserver.log" 2>&1 < /dev/null &
record APISERVER_PGID "$!"

STACK_URL="http://127.0.0.1:$HTTP_PORT"
log "waiting for the apiserver at $STACK_URL"
for _ in $(seq 1 120); do
  if curl -fsS -o /dev/null "$STACK_URL/" 2> /dev/null; then
    record STACK_URL "$STACK_URL"
    trap - ERR
    log "ready"
    echo "STACK_URL=$STACK_URL"
    exit 0
  fi
  kill -0 "$(sed -n 's/^APISERVER_PGID=//p' "$STATE_FILE")" 2> /dev/null || break
  sleep 1
done
tail -n 30 "$STATE_DIR/apiserver.log" >&2 || true
die "the apiserver did not start; see $STATE_DIR/apiserver.log"
