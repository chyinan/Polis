# Slice 233 — REQ-39 owner review state in Workbench

## Change

Mission change request event projections now return the Planning assessment ID, SHA-256, and risk level recorded when the owner considered the request and when the change was applied. The Workbench displays the current assessment SHA and hashes bound to lifecycle events. It compares the current receipt with the latest consideration and disables apply when the receipt is stale, missing, or has changed since owner review, with a prompt to reconsider while the Mission is paused.

This presents the server-side Slice232 apply fence before the operator submits the command. Existing event rows without assessment metadata remain readable; new metadata must be complete and valid.

## Verification

- `go build ./...`: passed.
- `go build -o .runtime/bin/polis ./cmd/polis`: passed; the local deterministic backend was restarted with this binary.
- `npm run build` (`tsc -b` and Vite production build): passed; Vite reports the existing large-chunk advisory.
- `git diff --check`: passed.
- Backend `/healthz`: HTTP 200, status `ready`.
- Frontend root: HTTP 200.
- Read-only PostgreSQL query: Schema 108; 0 WorkerSessions.
- Traceability JSON parses; evidence is linked from all eight REQ-39 scenario rows. Their dispositions remain `partial` and execution statuses remain `not_run`; all 232 frozen scenario states remain `not_run`.
- No tests, Worker/provider activity, Mission change request, or frozen scenario ran.

REQ-39 remains partial pending session-backed qualification and frozen scenarios.
