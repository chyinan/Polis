#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .runtime/windows/r0.3a-backend-l2-requalification
GOOS=windows GOARCH=amd64 bash scripts/go.sh build -o .runtime/windows/r0.3a-backend-l2-requalification/polis-r03a-backend-l2-requalification.exe ./cmd/polis-r03a-backend-l2-requalification
