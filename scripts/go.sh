#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOPATH="$PWD/.runtime/linux/gopath"
export GOCACHE="$PWD/.runtime/linux/go-cache"
export GOTMPDIR="$PWD/.runtime/linux/go-tmp"
export GOPROXY="${GOPROXY:-https://goproxy.cn}"
mkdir -p "$GOTMPDIR"
exec "$PWD/.tools/go/bin/go" "$@"
