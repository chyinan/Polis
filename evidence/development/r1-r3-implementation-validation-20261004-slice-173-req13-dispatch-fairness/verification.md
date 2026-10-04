# Slice 173 — REQ-13 durable dispatch fairness

## Changes

- Added Schema 102 `product_worker_dispatch_state` and `product_worker_dispatch_claims`. A singleton cursor row is locked for each claim transaction. Candidate selection, cursor advancement and the 30-second per-Task lease are committed together.
- Candidate queries wrap to the beginning when no Company remains after the persisted cursor and ignore nonexpired claims. Expired claims are deleted in the transaction, allowing process-crash recovery. Claim IDs are random 128-bit lowercase hex values.
- Replaced Control's process-local cursor with the database claim API. The dispatch path releases a claim after Worker startup returns; if release fails, the database expiry clears it. The Worker's existing admission path still rechecks Task/Company/Mission state and creates the authoritative WorkerSession before activation and provider turn execution.
- Schema 102 is forward-only: downgrading/resetting coordination state could remove a live claim and allow duplicate admission.

## Verification

- `go build ./...` — passed.
- `go build -o .runtime/bin/polis ./cmd/polis` — passed; rebuilt the local development CLI.
- `bash .runtime/dev-termux.sh db-start` — local PostgreSQL started.
- `bash .runtime/dev-termux.sh migrate` — applied Schema 102.
- Read-only PostgreSQL state query — `Schema=102, Companies=0, WorkerSessions=0, cursor rows=1, active claims=0`.
- `sha256sum -c db/migration_hashes.sha256` from `db/migrations/` — all 102 pinned migrations passed.
- `git diff --check` — passed.

No tests were run. The dispatcher itself was not iterated, and no Worker or provider session/turn was started.

## Remaining boundary

This adds durable fair Task claims, not an organization-wide concurrency limit. No authoritative global slot cap or provider quota-readiness signal exists in current runtime configuration. The frozen team-capacity draft cannot supply that authority, so quota readiness remains fail-closed and global slot enforcement remains open under REQ-13.
