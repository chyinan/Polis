#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local GOPROXY=off GOFLAGS=
artifact="$PWD/bin/polis-selfdev-go-host-probe"
mkdir -p "$PWD/bin"
bash scripts/go.sh build -trimpath -o "$artifact" ./cmd/polis-selfdev-go-host-probe
[[ "$("$artifact" --version)" == 'polis-selfdev-go-host-probe@1' ]]
printf 'host_probe_built=yes\nartifact=%s\nsha256=%s\n' "$artifact" "$(sha256sum "$artifact" | awk '{print $1}')"
