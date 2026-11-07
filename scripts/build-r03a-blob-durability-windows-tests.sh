#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
GOOS=windows GOARCH=amd64 bash scripts/go.sh test -c ./internal/kernel -o .runtime/windows/r0.3a-blob-durability/polis-kernel-tests.exe
