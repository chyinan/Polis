#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .runtime/windows/r0.3a-pagination-v3
GOOS=windows GOARCH=amd64 bash scripts/go.sh build -o .runtime/windows/r0.3a-pagination-v3/polis-r03a-pagination-v3.exe ./cmd/polis-r03a-pagination-v3
