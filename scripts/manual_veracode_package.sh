#!/usr/bin/env bash
# One-off manual Veracode packaging for JetStore (no Docker).
# Run on a Linux Jenkins agent (e.g. HBA-Analytics) after checking out this repo.
#
# Example:
#   cd /path/to/repo_infra_jetstore_platform
#   sudo ./scripts/manual_veracode_package.sh
#
# Output:
#   /jenkins/tmp/veracode-manual/jetstore.zip
#   /jenkins/tmp/veracode-manual/veracode-package.log

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR=/app
WORK=/jenkins/tmp/verascan-manual
OUTPUT_DIR=/jenkins/tmp/veracode-manual
LOG_FILE="${OUTPUT_DIR}/veracode-package.log"

VERACODE_CLI="${VERACODE_CLI:-/home/tomcat/veracode}"
if [[ ! -x "$VERACODE_CLI" ]]; then
  VERACODE_CLI="$(command -v veracode || true)"
fi
if [[ -z "$VERACODE_CLI" ]]; then
  echo "ERROR: veracode CLI not found. Set VERACODE_CLI or install the CLI." >&2
  exit 1
fi

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "ERROR: run this on Linux (Jenkins agent). macOS paths/CGO differ." >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: go not on PATH. Install Go 1.26.4 and retry." >&2
  exit 1
fi

for tool in gcc g++ zip unzip; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "ERROR: missing required tool: $tool" >&2
    exit 1
  fi
done

mkdir -p "$OUTPUT_DIR"
exec > >(tee -a "$LOG_FILE") 2>&1

echo "=== JetStore manual Veracode package ==="
echo "Started: $(date -Is)"
echo "Repo:    $REPO_ROOT"
echo "Go:      $(go version)"
echo "CLI:     $("$VERACODE_CLI" version 2>/dev/null || "$VERACODE_CLI" --version 2>/dev/null || echo unknown)"

rm -rf "$APP_DIR"
mkdir -p "$APP_DIR/cdk/jetstore_one"
cp "$REPO_ROOT/go.mod" "$REPO_ROOT/go.sum" "$APP_DIR/"
cp -a "$REPO_ROOT/jets" "$APP_DIR/"
cp -a "$REPO_ROOT/cdk/jetstore_one/lambdas" "$APP_DIR/cdk/jetstore_one/"

export GOWORK=off
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64
export VERACODE_PACKAGE_GOLANG_GENERATE=on
export GOCACHE="${GOCACHE:-/jenkins/tmp/go-build}"
export GOMODCACHE="${GOMODCACHE:-/jenkins/tmp/go-mod}"
mkdir -p "$GOCACHE" "$GOMODCACHE"

cd "$APP_DIR"
echo "Downloading modules..."
go mod download

rm -rf "$WORK"
mkdir -p "$WORK"

echo "Running: $VERACODE_CLI package --source $APP_DIR --output $WORK --trust --strict --verbose"
set +e
"$VERACODE_CLI" package --source "$APP_DIR" --output "$WORK" --trust --strict --verbose
PKG_RC=$?
set -e

ART="$(ls "$WORK"/veracode-auto-pack-*-go.zip 2>/dev/null | head -n1 || true)"
if [[ -n "$ART" ]]; then
  cp "$ART" "$OUTPUT_DIR/jetstore.zip"
  echo "SUCCESS: $OUTPUT_DIR/jetstore.zip"
  unzip -l "$OUTPUT_DIR/jetstore.zip" | head -30
  exit 0
fi

echo "ERROR: no Go artifact produced (exit $PKG_RC)"
echo "Work dir contents:"
ls -la "$WORK" || true
echo "Full log: $LOG_FILE"
exit "${PKG_RC:-1}"
