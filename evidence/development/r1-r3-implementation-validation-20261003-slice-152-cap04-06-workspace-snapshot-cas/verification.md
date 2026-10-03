# Slice 152 — CAP-04–06 workspace epochs, recovery snapshot and CAS retention

Date: 2026-10-03

## Scope

Audit and close the recovery-cut consistency gap found while tracing CAP-04–06. No migration, live provider call, external communication, test suite or database-backed Task run was performed.

## Source findings

- `db/migrations/00002_workers.sql` constrains each Company+Employee to one non-stopped WorkerSession. Task execution is checked against the persisted Task owner and WorkerSession; product workspace replacement additionally requires the current working Task and compares both expected digest and expected monotonic revision in `internal/kernel/worker_state.go`.
- `internal/kernel/cas_collection.go` registers 24-hour claims for CAS-first writes. Collection holds the per-Company CAS lifecycle advisory lock and Company row lock while checking durable references and claims and removing only unreferenced digest files. Unknown entries and failed reads are rejected.
- Schema 78 adds immutable `memory_cas_retention_pins` for historical memory references. Artifact rows and other company-owned database rows remain visible to the collector's reference scan.
- `PeerHandoverBoundarySnapshot` previously read recovery anchors, message, contract, workspace, Mission snapshot and CAS inventory independently. This could combine database state from different commits even though the individual records were verified.

## Change

`internal/kernel/peer_collaboration.go` now acquires the CAS lifecycle lock and then locks the Company row before resolving anchors. It reads the message, contract, workspace bytes and Mission snapshot through that transaction, inventories and verifies CAS while both locks remain held, performs the existing anchor checks, then commits. This lock order matches the collector and CAS-first writer protocol. `internal/kernel/recovery.go` factors the Mission snapshot query so it can participate in the same locked transaction; standalone `Snapshot` retains its repeatable-read read-only transaction.

CAP-04 workspace mutation continues to require the current database-backed WorkerSession/task-owner binding and digest+revision CAS. CAP-05 workspace and Artifact reads are content-addressed and validate expected digest/size; unavailable or inconsistent bytes return errors instead of partial content. CAP-06 collection honors durable claims, persisted references and immutable memory pins; recovery-cut contents and CAS inventory now share a lock boundary. These source checks do not qualify OS-level host access or prove behavior under a live database race.

## Verification

- `go build ./cmd/polis ./internal/kernel ./internal/control ./internal/provider ./internal/codex` — passed.
- `git diff --check` — passed.
- Go tests, live PostgreSQL concurrency qualification, live WorkerSession E2E, and frozen CAP-04–06 scenarios — not run. The frozen scenario catalog is unchanged; exact execution states remain `not_run`.
- Local dev database remains at Schema 98; this change does not use it or alter it.

## Files

- `internal/kernel/peer_collaboration.go`
- `internal/kernel/recovery.go`
- `docs/implementation/PROGRESS.md`
- `docs/implementation/NEXT_SLICE.md`
- `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`
