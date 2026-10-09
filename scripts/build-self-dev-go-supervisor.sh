#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { printf 'linux amd64 build host required\n' >&2; exit 2; }
artifact="$PWD/bin/polis-selfdev-go-job-supervisor"
mkdir -p "$PWD/bin"
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local GOPROXY=off GOFLAGS=
bash scripts/go.sh build -trimpath -o "$artifact" ./cmd/polis-selfdev-go-job-supervisor
source_identity="$("$artifact" --source-identity "$PWD")"
[[ "$source_identity" =~ ^[a-f0-9]{64}$ ]] || exit 2
bash scripts/go.sh build -trimpath -ldflags "-X polis/internal/gotestsupervisor.SourceIdentity=$source_identity" -o "$artifact" ./cmd/polis-selfdev-go-job-supervisor
"$artifact" --protocol-info > "$artifact.identity.json"
"$artifact" --verify-build "$artifact.identity.json" --source-root "$PWD"
printf 'supervisor_binary_built=yes\nartifact=%s\nidentity=%s.identity.json\nprivileged_apply_authorized=no\nhost_execution_profile_qualified=no\n' "$artifact" "$artifact"
