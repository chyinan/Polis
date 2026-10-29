#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
export GOPATH="$repo_root/.runtime/linux/gopath"
export GOCACHE="$repo_root/.runtime/linux/go-cache"
export GOTMPDIR="${POLIS_GO_TMP_ROOT:-$repo_root/.runtime/linux}/go-tmp"
export TMPDIR="${POLIS_GO_TMP_ROOT:-$repo_root/.runtime/linux}/test-tmp"
export GOPROXY="${GOPROXY:-https://goproxy.cn}"
mkdir -p "$GOTMPDIR" "$TMPDIR"
cd "$repo_root/third_party/giraffesyo-pdf"
exec "$repo_root/.tools/go/bin/go" test -p 1 "$@"
