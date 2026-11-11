#!/usr/bin/env bash
# Builds the JetStore binaries the screenshot stack runs, from this checkout,
# into JETS_BIN_DIR.
#
#   JETS_BIN_DIR=/path/to/bin docs/screenshots/stack/build-bin.sh
#
# Several checkouts can share one JETS_BIN_DIR: build it once, then point every
# run at it. The directory holds the Go binaries, the files they read beside
# them (jets_schema.json, workspace_schema.sql, the init script), the workspace
# document validator, and BUILD_INFO naming the commit they were built from.
#
# Never point JETS_BIN_DIR at /usr/local/bin: what is there belongs to whatever
# was last installed on the machine, not to this checkout.
set -euo pipefail

: "${JETS_BIN_DIR:?set JETS_BIN_DIR to the directory the binaries go to}"
case "$JETS_BIN_DIR" in
  /usr/local/bin|/usr/local/bin/) echo "JETS_BIN_DIR must not be /usr/local/bin" >&2; exit 2 ;;
esac

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
mkdir -p "$JETS_BIN_DIR"
BIN="$(cd "$JETS_BIN_DIR" && pwd)"

echo "Building the JetStore binaries from $REPO_ROOT into $BIN"
cd "$REPO_ROOT"
for target in \
  apiserver=./jets/apiserver \
  update_db=./jets/update_db \
  run_reports=./jets/run_reports \
  compile_workspace=./jets/cmds/compile_workspace \
  install_workspace_assets=./jets/cmds/install_workspace_assets; do
  name="${target%%=*}"
  pkg="${target#*=}"
  echo "  go build $name"
  go build -o "$BIN/$name" "$pkg"
done

# The rules unit test harness runs the C++ rule engine, so it links libjets.so.
# The engine is built in a tree of its own under JETS_BIN_DIR, and the stack
# puts JETS_BIN_DIR on LD_LIBRARY_PATH.
echo "  building libjets.so (C++ rule engine)"
mkdir -p "$BIN/cmake-build"
cmake -S "$REPO_ROOT" -B "$BIN/cmake-build" -DCMAKE_BUILD_TYPE=Release > "$BIN/cmake-build/configure.log"
cmake --build "$BIN/cmake-build" --target jets -j "$(nproc)" > "$BIN/cmake-build/build.log"
cp "$BIN/cmake-build/jets/libjets.so" "$BIN/libjets.so"
echo "  go build rules_unit_test"
CGO_LDFLAGS="-L$BIN" go build -o "$BIN/rules_unit_test" ./jets/cmds/rules_unit_test

# The files the binaries read beside them on an installation.
cp jets/jets_schema.json "$BIN/jets_schema.json"
cp jets/workspace_schema.sql "$BIN/workspace_schema.sql"
# aws_s3 is an RDS extension; a plain PostgreSQL does not have it.
sed '/aws_s3/d' jets/jets_init_db.sql > "$BIN/jets_init_db.sql"

# The workspace document validator the compile runs (jetsclient_ide).
echo "  building the workspace document validator"
(
  cd jetsclient_ide
  [ -d node_modules ] || npm ci --no-audit --no-fund
  npm run --silent build:validator
)
cp jetsclient_ide/dist-validator/jets_validate_workspace.mjs "$BIN/"

{
  echo "commit=$(git -C "$REPO_ROOT" rev-parse HEAD)"
  echo "dirty=$(git -C "$REPO_ROOT" status --porcelain --untracked-files=no -- jets jetsclient_ide | wc -l)"
  echo "built=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$BIN/BUILD_INFO"
echo "Done: $(tr '\n' ' ' < "$BIN/BUILD_INFO")"
