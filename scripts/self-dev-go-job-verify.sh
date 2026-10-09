#!/usr/bin/env bash
# Trusted development checks only. Never dispatches candidate Go code.
set -euo pipefail
cd "$(dirname "$0")/.."
repo_root="$PWD"
evidence="$repo_root/evidence/development/polis-self-dev-v1-supervisor-pg-20261009/verification-runtime/$(date -u +%Y%m%dT%H%M%SZ)-$$"
mkdir -p "$evidence"
temp_root="$(mktemp -d /tmp/polis-self-dev-go-check.XXXXXX)"
cleanup() {
  local status=$?
  trap - EXIT
  case "$temp_root" in
    /tmp/polis-self-dev-go-check.*)
      if [[ -d "$temp_root" && ! -L "$temp_root" ]]; then
        find "$temp_root" -type d -user "$(id -u)" -exec chmod u+w {} +
        rm -rf -- "$temp_root"
      fi ;;
    *) status=1 ;;
  esac
  printf 'verification_exit=%s\n' "$status" >> "$evidence/status.txt"
  printf 'evidence_dir=%s\n' "$evidence"
  exit "$status"
}
trap cleanup EXIT
export POLIS_GO_TMP_ROOT="$temp_root" GOTOOLCHAIN=local GOFLAGS=
run_check() {
  local name="$1"; shift
  printf 'check=%s\n' "$name"
  local delimiter='' argument
  {
    for argument in "$@"; do
      printf '%s%q' "$delimiter" "$argument"
      delimiter=' '
    done
    printf '\n'
  } > "$evidence/$name.command"
  if "$@" > "$evidence/$name.log" 2>&1; then
    printf '%s=PASS exit=0\n' "$name" >> "$evidence/status.txt"
  else
    local status=$?
    printf '%s=FAIL exit=%s\n' "$name" "$status" >> "$evidence/status.txt"
    return "$status"
  fi
}
run_check supervisor-build bash scripts/build-self-dev-go-supervisor.sh
cp bin/polis-selfdev-go-job-supervisor.identity.json "$evidence/supervisor.identity.json"
run_check script-syntax bash -n scripts/build-self-dev-go-supervisor.sh scripts/self-dev-go-job-host.sh scripts/self-dev-go-job-host-root.sh scripts/self-dev-go-job-host-test.sh scripts/self-dev-go-job-pg-test.sh scripts/self-dev-go-job-verify.sh scripts/go.sh
run_check host-cli bash scripts/self-dev-go-job-host-test.sh
run_check host-inspect bash scripts/self-dev-go-job-host.sh inspect
run_check host-prepare bash scripts/self-dev-go-job-host.sh prepare
run_check go-tests bash scripts/go.sh test -p 1 -count=1 -json ./...
run_check go-race bash scripts/go.sh test -race -p 1 -count=1 -json ./...
run_check go-vet bash scripts/go.sh vet ./...
run_check linux-build env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 bash scripts/go.sh build ./cmd/...
run_check windows-build env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 bash scripts/go.sh build ./cmd/...
run_check postgres-full bash scripts/self-dev-go-job-pg-test.sh
printf 'provider_egress=0\nreal_model_worker_started=0\ncandidate_project_execution=NOT_RUN\nautomatic_merge=NO\nautomatic_deploy=NO\nhost_execution_profile_qualification=BLOCKED\n' >> "$evidence/status.txt"
