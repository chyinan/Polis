#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
run_root="${1:-preflight}"
out="$repo/evidence/development/r0.5b16-reservation-to-initialize-bridge-hardening/$run_root/runtime/polis-r05b16.exe"
mkdir -p "$(dirname "$out")"
export GOOS=windows
export GOARCH=amd64
exec bash "$repo/scripts/go.sh" build -o "$out" ./cmd/polis-r05b16
