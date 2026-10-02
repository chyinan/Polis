# Slice 116 — Workbench memory correction queue

Date: 2026-10-02

## Changes

- Added a Kernel repeatable-read projection for up to 100 latest memory correction requests per company, with an explicit truncation bit.
- The projection computes proposal status from the immutable review event, latest revision state, and the authoritative revocation overlay. It verifies the stored base and proposed content SHA-256 values before returning metadata.
- The response contains no memory text, content digest, or free-form proposer/reviewer rationale. It contains only bounded identifiers, revision/state metadata, source reference metadata, timestamps and fixed employee IDs.
- Added `GET /api/workbench/companies/{company}/memory/corrections`, marked `Cache-Control: no-store`, and a read-only Settings panel.
- Proposal and review writes remain unavailable from Workbench. Its owner token does not identify a fixed employee or carry a WorkerSession binding. Using `BindFake` with a caller-selected ID would write a false employee identity into immutable audit records. The existing Kernel proposal/review commands remain available to genuinely employee-bound Kernel callers.

## Verification

- `go build ./cmd/...` — passed.
- `cd frontend && npm run build` — passed. Vite emitted its existing large-chunk advisory (>500 kB).
- `git diff --check` — passed.
- No tests were added or run.
- No database migration or PostgreSQL runtime query was run. SQL shape was checked against the frozen Schema 75–80 definitions; behavioral query execution remains unqualified.
- No WorkerSession, provider, external service, or production action was started.

## Remaining boundary

A trusted employee-session-bound Workbench command path is still needed to expose proposal and independent-review writes without letting a local owner impersonate a fixed employee. Product-provider successor admission and the six frozen REQ-15 scenarios remain open.
