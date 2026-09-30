#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-1"
output="/mnt/c/Users/chyinan/AppData/Local/Temp/polis-r05b-live-1-20260917/polis.exe"
export GOOS=windows
export GOARCH=amd64
bash scripts/go.sh build -o "$output" ./cmd/polis > "$evidence/windows-build-preflight.txt" 2>&1
sha256sum "$output" >> "$evidence/windows-build-preflight.txt"
