# Slice 220 — REQ-39 lease-bound workspace reads

Added a read-only manifest endpoint and a per-file content endpoint under the Mission takeover lease route. Both require the Mission to remain paused, the lease to remain granted and to own the Task's active takeover slot, and the live private workspace tree to reproduce the exact Schema 105 binding. Each file read resolves only a path present in that pinned manifest and verifies the CAS bytes, digest-bound length, and UTF-8 before returning content. Manifest and file reads append company event records with the lease and frozen-manifest provenance. This gives the Workbench a safe read source for the following UI stage; the current single-file edit and return contract remains unchanged.

## Verification

- `go build ./...` passed.
- `git diff --check` passed.
- No tests were run. No database operation, takeover lease, WorkerSession, provider activity, or frozen scenario was run.

REQ-39 remains partial. The Workbench does not yet consume these endpoints, edit a multi-file tree, or submit a complete multi-file handback.
