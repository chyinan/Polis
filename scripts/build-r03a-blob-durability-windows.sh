#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
GOOS=windows GOARCH=amd64 bash scripts/go.sh build -o .runtime/windows/r0.3a-blob-durability/polis-r03a-blob-durability.exe ./cmd/polis-r03a-blob-durability
