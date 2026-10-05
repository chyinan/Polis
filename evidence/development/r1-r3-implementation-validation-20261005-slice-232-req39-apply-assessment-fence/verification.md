# Slice 232 — REQ-39 apply-time Planning assessment fence

## Change

The owner can resume a Mission after considering a change request, allowing Planning to refresh its assessment against a new basis. Before this slice, the final apply command checked only the deterministic impact digest and safe boundary; it did not ensure the latest Planning receipt was still current or matched the receipt the owner considered. A refreshed high-risk receipt could therefore supersede the reviewed low-risk receipt while the request remained in `considered` state.

The final apply transaction now validates the current immutable Planning assessment against the locked request and current impact, requires the exact assessment SHA from the latest owner consideration event, and rechecks that high or uncertain risk has `BlockPreviousResults` enabled. The existing requirements/input digest, impact digest, paused-Mission, stopped-writer and takeover-lease checks remain in the same transaction. A changed assessment must be considered again before application.

## Verification

- `go build ./...`: passed.
- `go build -o .runtime/bin/polis ./cmd/polis`: passed; the local deterministic backend was restarted with this binary.
- `git diff --check`: passed.
- Backend `/healthz`: HTTP 200, status `ready`.
- Frontend root: HTTP 200.
- Read-only PostgreSQL query: Schema 108; 0 WorkerSessions.
- Traceability JSON parses; this evidence is linked from all eight REQ-39 scenarios. Their dispositions remain `partial` and execution statuses remain `not_run`; all 232 frozen scenarios remain `not_run`.
- No tests, Worker/provider activity, Mission change request, or frozen scenario ran.

REQ-39 remains partial pending session-backed qualification and frozen scenarios.
