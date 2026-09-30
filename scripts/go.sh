#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOPATH="$PWD/.runtime/linux/gopath"
export GOCACHE="$PWD/.runtime/linux/go-cache"
export GOTMPDIR="$PWD/.runtime/linux/go-tmp"
export TMPDIR="$PWD/.runtime/linux/test-tmp"
export POLIS_MIGRATION_ATTEMPT_ROOT="${POLIS_MIGRATION_ATTEMPT_ROOT:-$GOTMPDIR/migration-attempts}"
export GOPROXY="${GOPROXY:-https://goproxy.cn}"
mkdir -p "$GOTMPDIR" "$TMPDIR"
exec "$PWD/.tools/go/bin/go" "$@"
