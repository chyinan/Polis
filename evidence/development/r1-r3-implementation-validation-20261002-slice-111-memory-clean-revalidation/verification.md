# Slice 111 verification — memory clean-context revalidation

Date: 2026-10-02

## Changes

- Schema 77 adds an immutable revalidation evidence ledger and allows the trusted local control path to bind a corrected dependency to a Task without claiming that an old WorkerSession consumed it.
- The revalidation preview binds the correction and still-usable source, current replacement revision, Task plan digest/generation, workspace digest/revision, target digest, stopped source session, and remaining impacts.
- Confirmation rechecks those facts under the company lock, rejects terminal Tasks/Missions, inactive Companies, changed targets, stale corrections, missing/corrupt workspace CAS, or any non-stopped Task WorkerSession.
- A confirmed revalidation inserts an immutable replacement dependency, marks the old dependency and Task impact revalidated, records the operator reason/context digest, and returns a stopped `working` Task to `ready` with an incremented generation.
- Worker Handover now includes current scope-authorized memory dependency content (at most 16 records and 128 KiB) and approved replacement content for outstanding impacts. Unreadable or oversized context fails closed.

## Verification performed

- `gofmt` on changed Go files: passed.
- `git diff --check`: passed.
- `go build ./cmd/...`: passed (exit 0).
- Full migration checksum manifest verification: passed through Schema 77.

## Not run

- No Go tests were run.
- Schema 77 was not executed against PostgreSQL; no database migration runtime was run.
- No model/provider turn, Worker, external endpoint, or production action was used.

## Remaining work

- Expose owner-authorized revalidation preview/confirm commands through the Workbench and qualify the operator flow.
- Product-provider Tasks still cannot admit a successor WorkerSession after their first attempt; safe successor admission and its retry/budget policy remain open.
- Add reference-aware content/CAS retention pins and concurrent garbage-collection protection.
- Add durable deletion/revocation overlays that survive restoration of older backups.
- FT-37/40/41 and the associated acceptance scenarios remain partial or `not_run`; this slice does not claim REQ-15 completion.
