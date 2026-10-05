# Slice 239 — REQ-02 fixed roles from Company creation

## Change

`TEAM_COVERAGE.json` declares `role_changes_at_runtime: false`, but the previous update path only locked role names after owner acknowledgment, and Company creation accepted arbitrary non-empty role labels. The fixed four Employee IDs now have canonical role names required on Company creation. An existing-Company owner acknowledgment validates the persisted enabled roster inside its company-guarded transaction before appending the event. Every Company update compares incoming role names with the persisted enabled roster and denies changes regardless of acknowledgment state. Display names, model profiles, Company name, and workspace path remain editable. Company detail and switcher read models report the matrix as confirmed only when the latest embedded-template digest and persisted canonical roles both match. Qualification remains `unverified`; execution remains disabled.

The local database has no Company rows, so no role mutation or owner confirmation was attempted. Existing persisted mismatched role assignments remain visible as unconfirmed and cannot be owner-confirmed through this path until they receive owner-reviewed reconciliation.

## Verification

- `gofmt -w internal/organization/organization.go internal/kernel/organization.go` completed.
- `go build ./...` passed.
- `go build -o .runtime/bin/polis ./cmd/polis` passed.
- The rebuilt deterministic backend was restarted and returned `{"service":"polis_backend","status":"ready","version":"r0.7"}`.
- The Vite development frontend returned HTTP 200 at `127.0.0.1:4173`.
- Read-only PostgreSQL transaction reported latest applied Schema 108, zero Companies, zero Missions, and zero WorkerSessions, then completed with `ROLLBACK`.
- `git diff --check` passed.
- The seven REQ-02 scenario rows cite this evidence and remain `partial/not_run`; all 232 frozen scenarios remain `not_run`.
- No tests, owner acknowledgment, database writes, Worker/provider operations, or frozen scenarios ran.
