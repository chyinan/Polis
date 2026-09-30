# R1–R3 implementation validation — slice 8

Date: 2026-09-24

## Scope

Added `MaterializeNodeNPMProjectFiles`, a bounded, non-executing helper that copies an already verified directory/ZIP project snapshot into a new child of an application-managed destination root. It revalidates the immutable plan before writing, rejects traversal and Windows-invalid/device names, symlinked root/parent paths, pre-existing workspaces and stale package metadata; it creates files exclusively, hashes each written file again, then rechecks the package and lockfile hashes from disk. Failed materialization removes only the workspace created by that call.

The helper is not connected to Control or the environment preparation state machine. It does not qualify filesystem/network isolation, invoke Node/npm, or start JobRuns.

## Verification

- `bash scripts/go.sh test -count=1 ./internal/environment` — passed.
- Materializer tests passed for successful snapshot materialization, file/package/lock revalidation, path traversal, reserved Windows device names, metadata drift, existing workspace refusal and symlinked destination roots.
- `bash scripts/go.sh test -count=1 ./...` — passed serially. Database-backed tests without `POLIS_TEST_DSN` used their explicit skip guards.
- `GOOS=linux GOARCH=amd64 bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh test -c -o .runtime/linux/go-tmp/environment.test.exe ./internal/environment` — passed as a cross-compile; the generated Windows test binary was not executed natively in this slice.
- `git diff --check` — passed.

## Boundaries

No Mission source was materialized through the product path, and no project process, Node/npm install, dependency network request, GitHub request, QQ send, model turn, MCP call, deployment or publication was run. AppContainer filesystem/network isolation, Control wiring, cancellation/recovery, JobRun logs and service readiness remain open.
