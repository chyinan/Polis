#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
for path in "$repo"/scripts/r0-5b-live5-*.sh; do
  bash -n "$path"
done
echo 'LIVE_5_BASH_SYNTAX_PASS'
