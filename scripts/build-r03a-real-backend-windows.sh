#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
GOOS=windows GOARCH=amd64 bash scripts/go.sh build -o .runtime/windows/r0.3a-real-backend/polis-r03a-real-backend.exe ./cmd/polis-r03a-real-backend
