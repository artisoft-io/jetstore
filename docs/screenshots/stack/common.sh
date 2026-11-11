# shellcheck shell=bash disable=SC2034
# Shared settings for the screenshot stack scripts. Sourced, not run.
#
# Everything a run creates is named for the checkout it runs from, so several
# checkouts on one workstation each get their own database container, ports
# and temporary directories, and never share one.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SHOTS_DIR="$REPO_ROOT/docs/screenshots"
STACK_DIR="$SHOTS_DIR/stack"

# A short, stable name for this checkout: its folder name and a hash of its path.
_checkout_hash="$(printf '%s' "$REPO_ROOT" | sha1sum | cut -c1-8)"
_checkout_base="$(basename "$REPO_ROOT" | tr -c 'a-zA-Z0-9_.-' '-' | cut -c1-30)"
STACK_NAME="jetstore-shots-${_checkout_base%-}-${_checkout_hash}"

# The state of a running stack: ports, process ids and logs. Git-ignored. The
# admin's password is never written here.
STATE_DIR="$SHOTS_DIR/.stack"
STATE_FILE="$STATE_DIR/stack.env"

# Short paths for what must be short (unix sockets) or stable (the workspace
# directory, whose path the screens can show).
RUN_DIR="${TMPDIR:-/tmp}/$STACK_NAME"

# The values every run uses, so the screens show the same thing every time.
STACK_WORKSPACE="jets_ws"
STACK_WORKSPACE_BRANCH="main"
STACK_JETS_VERSION="1767225600"            # 2026-01-01T00:00:00Z, as a release stamp
STACK_ADMIN_EMAIL="admin@localhost.test"

# The workspace the stack compiles: a jets_ws repository and the commit to take.
# JETS_WS_REPO defaults to the jets_ws checkout beside jetstore in a parent
# checkout that holds both.
JETS_WS_REPO="${JETS_WS_REPO:-$REPO_ROOT/../workspaces/jets_ws}"
JETS_WS_REF="${JETS_WS_REF:-789dd9b2}"

log() { printf '[stack] %s\n' "$*" >&2; }
die() { printf '[stack] error: %s\n' "$*" >&2; exit 1; }

# psql in the database container, reading SQL from stdin.
stack_psql() {
  docker exec -i "$STACK_NAME" psql -X -q -U postgres -d postgres -v ON_ERROR_STOP=1 "$@"
}
