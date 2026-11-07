#!/usr/bin/env bash
# pattern: Imperative Shell
set -euo pipefail

cd "$(dirname "$0")/.."
GOOS=windows GOARCH=amd64 bash scripts/go.sh build -o .runtime/windows/r0.3a-real-frontend/polis-r03a-real-frontend.exe ./cmd/polis-r03a-real-frontend
