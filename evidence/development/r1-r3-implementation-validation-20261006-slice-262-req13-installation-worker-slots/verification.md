# Slice 262 verification — REQ-13 installation-wide Worker slots

Date: 2026-10-06

## Scope

Schema 110 adds an installation-wide Worker slot policy with an intentionally unset cap/reserve, immutable revision events, and a durable reservation classification for each admitted WorkerSession. Existing non-stopped sessions are backfilled from their fixed Task class. Both production session insertion paths now lock the singleton policy row, count all non-stopped sessions across Companies, enforce the total and ordinary-slot limits, and persist the reservation in the same transaction as WorkerSession admission. A stopped session releases its capacity; stopping or reconciliation-required sessions continue to count.

An Installation Owner authenticated, CSRF-protected, revision-checked API and the Installation Owner page let the owner configure the total and protected slot counts. The interface does not choose defaults. When the policy is unset, admission fails closed with an actionable owner message. If a newly reduced cap is below current use, the read projection reports `over_capacity`; it does not stop existing work, and new admissions remain denied until capacity is available or the owner revises the policy. `review` and `peer_review` are currently the protected classes; project jobs and other kinds are ordinary.

This does not define provider-authoritative quota readiness or a release source for `waiting_quota`. No provider signal or slot number was inferred. The policy takes effect only after Schema 110 is applied; the local runtime and its last recorded database observation are still Schema 108. No migration was applied here.

## Verification

- `go build ./...`: passed.
- Migration manifest digest validation: all 110 SQL migration entries match their source files.
- `npm run build` in `frontend/`: passed (`tsc -b` and Vite production build); the existing >500 kB bundle advisory remains.
- `git diff --check`: passed.
- No tests or database operations were run. No WorkerSession, provider, or frozen scenario was used.
- All nine REQ-13 scenario rows remain `partial/not_run`; all 232 scenario executions remain `not_run`. The 17 open software requirement IDs are unchanged.
