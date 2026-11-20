#!/bin/bash
# Build the rule engine's shared library, libjets.so, from this checkout, and
# print the environment that makes Go link and load that library rather than
# another one.
#
# Why it is needed. A Go package that imports jets/bridge links libjets through
# cgo, and jets/bridge/bridge.go searches build/jets in this checkout, then the
# image paths /usr/local/go/build/jets and /app/jets, then the linker's default
# path. A checkout with no build/jets of its own (a git worktree, typically) falls
# through to that default path, and a libjets.so found there from an older
# checkout links without a word until a newer function is missing:
#
#   undefined reference to `get_vertex_observations'
#
# Even when the link succeeds, the test binary loads libjets.so at run time from
# the loader's path, not from where it was linked, so LD_LIBRARY_PATH must name
# the same directory.
#
# Usage, from anywhere in the checkout:
#   eval "$(utils/libjets_env.sh)"            # build into build/, export the env
#   eval "$(utils/libjets_env.sh -B bld)"     # another build directory (ignored)
#   utils/libjets_env.sh --no-build -B bld    # print the env for an existing build
#   go test -count=1 ./jets/compute_pipes/
#
# The build directory is relative to the checkout's root unless absolute. It is
# configured as a Release build the first time; an existing one is rebuilt with
# its own settings. Only the env lines go to stdout; the build's output goes to
# stderr. Needs CMake, a C++ compiler, SQLite and Abseil, as the engine's build
# does.
set -euo pipefail

usage() {
  echo "usage: $0 [-B <build dir>] [--no-build]" >&2
  exit 2
}

build_dir=build
do_build=1
while [ $# -gt 0 ]; do
  case "$1" in
    -B) [ $# -ge 2 ] || usage; build_dir="$2"; shift 2 ;;
    --no-build) do_build=0; shift ;;
    -h|--help) usage ;;
    *) usage ;;
  esac
done

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
case "$build_dir" in
  /*) ;;
  *) build_dir="$root/$build_dir" ;;
esac

if [ "$do_build" = 1 ]; then
  if [ ! -f "$build_dir/CMakeCache.txt" ]; then
    cmake -S "$root" -B "$build_dir" -DCMAKE_BUILD_TYPE=Release >&2
  fi
  cmake --build "$build_dir" --target jets -j "$(nproc)" >&2
fi

lib="$build_dir/jets"
if [ ! -f "$lib/libjets.so" ]; then
  echo "$0: no libjets.so in $lib; build it, or drop --no-build" >&2
  exit 1
fi

echo "export CGO_CFLAGS=\"-I$root/jets\""
echo "export CGO_LDFLAGS=\"-L$lib\""
echo "export LD_LIBRARY_PATH=\"$lib\${LD_LIBRARY_PATH:+:\$LD_LIBRARY_PATH}\""
