#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
out="$repo/evidence/development/r0.5b-real-provider-product-smoke-live-5/runtime/polis-live5.exe"
mkdir -p "$(dirname "$out")"
export GOOS=windows
export GOARCH=amd64
exec bash "$repo/scripts/go.sh" build -o "$out" ./cmd/polis
