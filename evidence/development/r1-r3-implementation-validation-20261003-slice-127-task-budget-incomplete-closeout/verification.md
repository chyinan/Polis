# Slice 127 verification — Task budget incomplete closeout

Date: 2026-10-03

## Scope

Schema 88 records an immutable local-owner decision to close a budget-blocked Task as incomplete. The record binds the latest budget-caused rejection and exact Task, ProblemKey and closing-reserve snapshots, including reserve remaining calls. It requires a nonterminal Task with no live WorkerSession. The Workbench displays rejection and closeout history and provides a reasoned, confirmed action. Future Worker admission and Task budget allocation are denied after closeout. Task row locking serializes admission, allocation and closeout; stale policy/counter snapshots make closeout ineligible until a fresh budget rejection is recorded.

Existing rejection history preserves the blocked outcome; the append-only ProblemKey/Task allocation paths provide owner-funded recovery. Slice 127 adds explicit incomplete closeout. This does not close REQ-16 as a whole.

## Verification

- `go build ./internal/kernel ./internal/control ./cmd/polis` — passed.
- `npm run build` from `frontend/` — passed; Vite reports the existing advisory for a minified bundle above 500 kB.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` — all 88 migration hashes passed.
- `git diff --check` — passed.
- Go tests were not run.
- PostgreSQL migration and runtime behavior were not run.

## Remaining REQ-16 gaps

Hidden provider/CLI retries are not fully visible to the protocol tool-call observer. Token/money accounting and Mission/Company/Provider budget composition remain open.
