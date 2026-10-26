# Slice 326 — REQ-36 dependency overlay validation

Date: 2026-10-26
Result: `PURE_MATERIALIZATION_BOUNDARIES_IMPLEMENTED_AND_OFFLINE_VERIFIED`; end-to-end DependencyChange materialization remains open.

## Implemented

- Added a deterministic package-manifest overlay that changes only exact semver-pinned entries in the root `dependencies` object. It rejects duplicate JSON keys, case-variant aliases of known manifest fields, non-string dependency values, category collisions, unsupported workspaces, and no-op proposals.
- Source file slices are copied; the base package manifest, lockfile, and other files are not mutated.
- Added lockfile-result validation requiring the executor's package manifest to equal the deterministic overlay byte-for-byte. Lockfile duplicate/case-variant fields are rejected, then the existing Node project validator enforces lockfile version, root dependency equality, safe package paths, HTTPS registry tarballs, integrity metadata, profile bounds, and the proposal's registry subset of the base policy.
- These are pure helpers only. No npm invocation, package resolution, CAS write, MissionInput creation, environment revision creation, UI/API command path, or durable materialization attempt was added.

## Verification

- Focused environment package tests: PASS.
- Full Go suite: PASS — `go-tests.log`.
- Full Go race suite: PASS — `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- `git diff --check`: PASS.
- No database migration or disposable PostgreSQL test was needed for this pure functional increment.
- No npm, registry, Provider, or external network request ran.

## Remaining REQ-36 work

Still required: owner-facing DependencyChange commands, a qualified isolated lockfile resolver, a durable single-attempt/recovery state machine, immutable derived MissionInput storage, and a successor environment revision linked to the approved proposal. Linux's current profile is deny-all; registry resolution must remain unavailable there unless a separately approved and qualified offline-cache path is implemented. Windows registry-only execution and host qualification remain unverified.
