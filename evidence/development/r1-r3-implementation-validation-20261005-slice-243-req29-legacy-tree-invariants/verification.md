# Slice 243 — REQ-29 legacy workspace writer bounds

## Change

The legacy `workspace_replace` compatibility path now applies the same logical workspace-tree invariants as the file-tree writer whenever a Task has a tree root. A shared transaction helper checks the 512-file limit, 16 MiB aggregate byte limit, and file/directory prefix conflicts. The compatibility path locks the root and verifies the bound WorkerSession/epoch before updating the legacy row and mirrored tree entry. Existing Task, session, revision, and workspace authorization remains in force.

This protects the Company/Task-scoped logical CAS tree only. It does not add host mounts, shared roots, retention policy, or CAP qualification.

## Verification

- `go build ./...` passed.
- `git diff --check HEAD^ HEAD` passed.
- Confirmed the shared helper is called by both the normal tree writer and legacy compatibility writer; bounds remain 512 files and 16 MiB.
- Traceability remains 17 open requirements; all 232 scenarios remain `not_run`.
- No tests, database operations, Worker/provider operation, host qualification, or frozen scenario ran.
