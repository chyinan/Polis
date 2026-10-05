# Slice 254 — REQ-29 legacy workspace/tree writer consistency

## Change

The legacy `TXReplace` path now checks any existing logical workspace root under lock. It verifies the private root, Task owner/Mission, and exact writer WorkerSession/epoch, then applies the same tree bounds and mirrors `formatter.go` into the logical tree and `worker_workspaces` in one transaction. The mirrored file records the updated legacy workspace revision as its source revision.

Product submission now fails closed unless a tree-backed compatibility workspace has exactly one `formatter.go` entry whose digest and source revision match the legacy workspace row. Historical Tasks without a tree retain their existing behavior. This remains a CAS-backed logical tree; it does not implement or qualify host filesystem mounts, OS symlink/special-file defenses, company/group shared roots, or CAP-01–06.

## Verification

- `go build ./...` passed on integrated source commit `9af5c33` (published implementation commit `8351042743bea775ba4f2db97d71de701cb740fd`).
- `git diff --check` passed.
- All six CAP-01–06 scenario rows remain `partial/not_run`; all 232 frozen scenarios remain `not_run`.
- No tests, database operation, Worker/provider action, host qualification, or frozen scenario ran.
