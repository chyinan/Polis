# Slice 216 — REQ-39 legacy/tree baseline fence

Date: 2026-10-05

## Change

Task takeover leases bind the legacy `worker_workspaces` digest and revision. After Schema 103 introduced a private file tree, that legacy row could cease to represent the complete Task tree. Granting a human lease in that state would expose only one text file as though it were the frozen Task workspace.

The Kernel now checks the Task tree at lease grant and again inside snapshot handback. If a tree exists, it must remain in the same Mission-private root and contain exactly one `workspace.txt` whose digest and `source_revision` match the legacy workspace row. Multi-file trees, the product `formatter.go` tree, and divergent files fail closed. Tasks without a Schema 103 tree retain the existing bounded single-file behavior. No migration or API shape change was needed.

## Verification

- `go build ./...`: passed.
- `git diff --check`: passed.
- No tests were added or run. No database operation, takeover lease/snapshot command, Worker/provider action, or frozen scenario ran.
- Frozen scenario execution states remain `not_run`.

## Status

REQ-39 remains partial. This closes the mismatch between the bounded legacy takeover route and an existing private Task tree. A lease-bound full tree manifest, multi-file patch/return, semantic impact assessment, and frozen-scenario qualification remain open.
