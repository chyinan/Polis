# Slice 334 — REQ-29 bounded read-only root snapshot primitive

Date: 2026-10-29
Result: `BOUNDED_OS_ROOT_SNAPSHOTTER_ADDED`; not connected to a live Company root.

## Implemented

- Added `intake.PrepareReadOnlyRootSnapshot(*os.Root)`. The API accepts an already-open root handle and no host path, and returns the existing canonical `directory_snapshot` package with the synthetic relative root label `workspace`.
- Traversal is finite and read-only. It applies existing directory snapshot caps (250 files, 7 MiB total, per-file upload limit), rejects observed symlinks and non-regular entries, checks file/directory identity before and after reads, and stores only relative file paths and CAS-ready bytes.
- The root source's absolute path is not returned or written into the package. Errors contain bounded reason codes rather than host paths.
- The primitive is not called by Control, Workbench or Worker code. No Company root binding or authority is created; the higher layer must still require explicit owner authorization and verify the previewed snapshot digest before persistence.

## Verification

- Intake tests: PASS — temp-root snapshot produces a canonical private package; symlink entry and absent root handle fail closed.
- Full Go suite, race suite and `go vet ./...`: PASS — `go-tests.log`, `go-race.log`, `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Frontend typecheck, ESLint, Vite build and Vitest: PASS — 19 files, 171 tests (`frontend-*.log`).
- Migration manifest test: PASS — `migration-manifest-test.log`; no migration was added.
- All filesystem tests used temporary test directories. No user workspace, business data, Worker, Provider or external network was accessed.

## Qualification still required

The code does not claim a no-follow or TOCTOU guarantee under concurrent host mutation, does not detect all filesystem/mount-boundary changes across platforms, and is not a native owner-authorized root binding. Do not wire it to Worker or Company root access until those native identity/race semantics and the owner authorization flow are implemented and qualified on the target host.
