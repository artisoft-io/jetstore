#!/bin/bash
# Compose the Go workspace a workspace's Go Lambdas are built under.
#
# A workspace module does not require the jetstore module: its imports of
# github.com/artisoft-io/jetstore resolve only through a go.work that uses both
# checkouts. The stack bundles a workspace Lambda (JETS_REGISTER_KEY_LAMBDA_ENTRY
# and the other *_LAMBDA_ENTRY variables) at synth, with `go build` run from the
# handler's module, and Go finds a go.work by walking up from there. So the
# go.work goes in the first directory above both checkouts, which is what this
# script writes. Dockerfile.cpipes_builder_ws does the same for the image entries.
#
# Usage:
#   utils/compose_workspace_go_work.sh [--check] <jetstore checkout> <workspace checkout>
#
# --check   change nothing: exit 0 when a build in the workspace resolves jetstore
#           through a go.work that uses both checkouts, 1 otherwise.
#
# It refuses, and changes nothing, when:
#   - the workspace has no go.mod at its root (a `use` of such a directory breaks
#     every go command under the go.work, not only the workspace's);
#   - one checkout is inside the other, so no directory is above both;
#   - a go.work between the workspace and that directory would be found first;
#   - that directory already holds a go.work that does not use both checkouts.
# A go.work already using both is left as it is.
set -euo pipefail

usage() {
  echo "usage: $0 [--check] <jetstore checkout> <workspace checkout>" >&2
  exit 2
}

check_only=false
if [ "${1:-}" = "--check" ]; then
  check_only=true
  shift
fi
[ $# -eq 2 ] || usage

die() {
  echo "error: $*" >&2
  exit 1
}

# Logical paths, as given: Go walks up from the directory the build runs in,
# which is the path the stack was given, not its symlink-resolved form.
abs_dir() {
  [ -d "$1" ] || die "not a directory: $1"
  (cd "$1" && pwd)
}

jetstore_dir=$(abs_dir "$1")
ws_dir=$(abs_dir "$2")

grep -qE '^module[[:space:]]+github\.com/artisoft-io/jetstore[[:space:]]*$' "${jetstore_dir}/go.mod" 2>/dev/null ||
  die "${jetstore_dir} is not a jetstore checkout (no go.mod for module github.com/artisoft-io/jetstore)"
[ -f "${ws_dir}/go.mod" ] ||
  die "${ws_dir} has no go.mod: a workspace Lambda needs a Go module at the workspace root"

case "${ws_dir}/" in "${jetstore_dir}/"*) die "${ws_dir} is inside ${jetstore_dir}: jetstore's own go.work would govern the build" ;; esac
case "${jetstore_dir}/" in "${ws_dir}/"*) die "${jetstore_dir} is inside ${ws_dir}: no directory is above both" ;; esac

# The first directory above both checkouts.
target="${ws_dir}"
while :; do
  target=$(dirname "${target}")
  case "${jetstore_dir}/" in "${target%/}/"*) break ;; esac
done
go_work="${target%/}/go.work"

# A go.work nearer the workspace would be found first.
dir="${ws_dir}"
while [ "${dir}" != "${target}" ]; do
  [ ! -f "${dir}/go.work" ] || die "${dir}/go.work would govern the workspace's build before ${go_work}; remove it or compose there"
  dir=$(dirname "${dir}")
done

# True when the go.work uses both checkouts, by path relative to its directory or absolute.
uses_both() {
  local used
  used=$(cd "${target}" && go work edit -json "${go_work}" |
    sed -n 's/.*"DiskPath": "\(.*\)".*/\1/p' |
    while read -r p; do
      case "$p" in /*) (cd "$p" 2>/dev/null && pwd) ;; *) (cd "${target}/$p" 2>/dev/null && pwd) ;; esac
    done)
  grep -qxF "${jetstore_dir}" <<<"${used}" && grep -qxF "${ws_dir}" <<<"${used}"
}

# What a build in the workspace resolves.
verify() {
  local gowork jetstore_resolved
  gowork=$(cd "${ws_dir}" && go env GOWORK)
  [ "${gowork}" = "${go_work}" ] || { echo "a build in ${ws_dir} uses GOWORK='${gowork}', not ${go_work}" >&2; return 1; }
  jetstore_resolved=$(cd "${ws_dir}" && go list -m -f '{{.Dir}}' github.com/artisoft-io/jetstore 2>/dev/null) ||
    { echo "a build in ${ws_dir} does not resolve github.com/artisoft-io/jetstore" >&2; return 1; }
  [ "$(cd "${jetstore_resolved}" && pwd -P)" = "$(cd "${jetstore_dir}" && pwd -P)" ] ||
    { echo "github.com/artisoft-io/jetstore resolves to ${jetstore_resolved}, not ${jetstore_dir}" >&2; return 1; }
}

if [ -f "${go_work}" ]; then
  uses_both || die "${go_work} exists and does not use both ${jetstore_dir} and ${ws_dir}; edit it with 'go work use', or remove it"
  echo "${go_work} already uses both checkouts"
elif ${check_only}; then
  echo "no ${go_work}" >&2
  exit 1
else
  # Paths relative to the go.work, so the directory can be moved as a whole.
  # GOWORK names the file to write: without it, `go work init` takes a go.work
  # found further up as the one to create, and refuses because it exists.
  (cd "${target}" && GOWORK="${go_work}" go work init "./${jetstore_dir#"${target%/}/"}" "./${ws_dir#"${target%/}/"}")
  echo "wrote ${go_work}:"
  cat "${go_work}"
fi

verify || exit 1
echo "a build in ${ws_dir} resolves github.com/artisoft-io/jetstore to ${jetstore_dir}"
