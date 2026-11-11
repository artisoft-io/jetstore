#!/usr/bin/env bash
# Runs the apiserver inside a network namespace of its own. Started by up.sh
# through `unshare -rn`; not meant to be run by hand.
#
# Why: with -usingSshTunnel the apiserver always listens on :8080, so two
# stacks on one workstation would collide. In its own namespace each apiserver
# has its own :8080, and two unix sockets carry the traffic across:
#   pg.sock    the namespace's 127.0.0.1:5432 -> the database container
#   http.sock  the host's free port           -> the namespace's :8080
# up.sh runs the host ends of both.
set -euo pipefail

: "${SOCK_DIR:?}" "${JETS_BIN_DIR:?}"

ip link set lo up

socat TCP-LISTEN:5432,bind=127.0.0.1,fork,reuseaddr "UNIX-CONNECT:$SOCK_DIR/pg.sock" &
socat "UNIX-LISTEN:$SOCK_DIR/http.sock,fork,unlink-early" TCP:127.0.0.1:8080 &

exec "$JETS_BIN_DIR/apiserver" \
  -usingSshTunnel \
  -adminEmail "$STACK_ADMIN_EMAIL" \
  -IDE_APP_DEPLOYMENT_DIR "$IDE_DIST" \
  -DOCS_DEPLOYMENT_DIR "$DOCS_DIST"
