#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
GOOS=windows GOARCH=amd64 bash scripts/go.sh build -o .runtime/windows/r0.3a-h1-review/polis-r03a-h1-review.exe ./cmd/polis-r03a-h1-review
