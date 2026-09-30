# Slice 25 verification — local fixed-commit Git snapshot reaches Worker

Date: 2026-09-25

## Changes covered

- Added `polis import-git COMPANY MISSION REPOSITORY_PATH FULL_COMMIT_SHA REQUEST_ID [INPUT_ID]` for explicit local import into an existing Mission.
- Import requires a full 40- or 64-character commit object ID already present in a local non-bare repository. It uses only read-only tree/blob object commands and does not scan worktree status, fetch, checkout, update refs, execute hooks/filters, use credential helpers, or contact configured remotes.
- Added canonical `git_snapshot` packaging with a repository identity digest, commit/tree IDs, sorted file hashes, per-path omissions, a `not_included_by_policy` worktree marker, and a bounded `.polis-git-source.json` Worker input. The importer does not call `git status` and never reads worktree files.
- Connected `git_snapshot` to CAS verification, immutable Task manifests, Worker context reconstruction, input delivery receipts, and Workbench archive-source validation. Added frontend validation for partial Git candidates and the existing R2 Linux/Node profile's offline `deny_all` policy.
- No schema migration was required; Schema 28 already admitted `git_snapshot`.

## Verification results

| Command / evidence | Result |
|---|---|
| RED test run for local Git/package limits: `bash scripts/go.sh test ./internal/intake -count=1` before implementation | FAIL as expected because `PrepareGitSnapshot`, `ImportGitCommit`, and snapshot metadata did not yet exist. |
| `bash scripts/go.sh test ./internal/intake ./internal/control ./cmd/polis -count=1` | PASS. Includes local Git repository creation, exact-commit import, dirty tree omission, an unreachable configured remote fixture, canonical package verification and source-note omission examples. |
| `POLIS_TEST_DSN='<dedicated schema-28 runtime DSN>' bash scripts/go.sh test ./internal/control -run '^TestProductWorkerReceivesFrozenGitSnapshotFromSelectedCommit$' -count=1` | PASS on dedicated PostgreSQL 18.6 / schema 28. Fake Worker received selected-commit README content and source provenance, did not receive dirty-worktree bytes, and the append-only final receipt contained the source and provenance refs with `provider_egress=0`. |
| `POLIS_TEST_DSN='<dedicated schema-28 runtime DSN>' bash scripts/go.sh test -race ./internal/intake ./internal/control -run Git -count=1` | PASS. |
| `bash scripts/go.sh test ./... -count=1` | PASS. Database-backed tests without a DSN were skipped in this full-suite run. |
| Frontend `npm test`, `npm run typecheck`, `npm run lint`, `npm run build` | PASS; 66 tests. Existing >500 kB chunk advisory remains. |
| `bash scripts/go.sh build ./cmd/polis ./cmd/polisd` and `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/polis ./cmd/polisd` | PASS. |
| `git diff --check` | PASS. |

The dedicated database initialization initially used a name rejected by Polis's non-R0 database guard; that attempt is preserved in `database-name-guard-attempt.log`. The corrected disposable database used the required `polis_r0_` prefix and migrated through schema 28. PostgreSQL setup/migration/runtime grants and the Worker/race test logs are preserved in this directory. The instance was stopped after verification and its exact `/tmp/polis-r1-s25.*` temporary root was removed.

## Not qualified

No real model call, remote Git fetch, GitHub account, submodule/LFS retrieval, project code execution, external MCP, QQ send, production database, or deployment was used. The Worker/runtime in the database end-to-end test was fake; this is local input/receipt evidence, not live provider qualification.
