# Slice 238 — REQ-02 confirmed-role immutability

## Change

The exact embedded `TEAM_COVERAGE.json` acknowledgment now activates the fixed role-name constraint in the Company update transaction. The transaction reads the latest acknowledgment event and only enforces the lock when its template digest matches the current embedded matrix. It then compares persisted enabled employee IDs and roles with the incoming roster. Changed membership or a changed role name is denied; display names and model profiles may still change. Company name and workspace root updates remain available. This prevents a later Company update from silently contradicting the owner-acknowledged fixed matrix while preserving the existing unverified qualification and execution-disabled state.

## Verification

- `gofmt -w internal/kernel/organization.go` completed.
- `go build ./...` passed.
- `go build -o .runtime/bin/polis ./cmd/polis` passed.
- The rebuilt deterministic backend was started and returned `{"service":"polis_backend","status":"ready","version":"r0.7"}`.
- The Vite development frontend returned HTTP 200 at `127.0.0.1:4173`.
- Read-only PostgreSQL transaction reported latest applied Schema 108, zero Companies, zero Missions, and zero WorkerSessions, then completed with `ROLLBACK`.
- `git diff --check` passed.
- The seven REQ-02 scenario rows cite this evidence and remain `partial/not_run`; all 232 frozen scenarios remain `not_run`.
- No tests, owner acknowledgment, database writes, Worker/provider operations, or frozen scenarios ran.
