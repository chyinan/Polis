# Slice 231 — REQ-39 Planning context snapshot consistency

## Change

`mission_change_request_read` now selects the unique open request and reads its immutable terms, latest lifecycle event, stored impact receipt, current Mission requirements/inputs, current impact, and latest Planning assessment within one read-only repeatable-read transaction. More than one open request fails as an integrity error. A request whose pinned base no longer matches current requirements or inputs returns a conflict before exposing an assessment basis. This avoids composing a closed or stale request with data from a later database snapshot.

The assessment write path still rechecks the current request, basis digest, active Planning WorkerSession, employee epoch, and working Planning-owned Task. No worker-session admission or authorization rule was relaxed.

## Verification

- `go build ./...`: passed.
- `go build -o .runtime/bin/polis ./cmd/polis`: passed; the local deterministic backend was restarted with this binary.
- `git diff --check`: passed.
- `GET http://127.0.0.1:8080/healthz`: HTTP 200, status `ready`.
- `GET http://127.0.0.1:4173/`: HTTP 200.
- Read-only PostgreSQL query: Schema 108; 0 Companies, 0 Missions, and 0 WorkerSessions.
- Traceability JSON parses; Slice 231 evidence is linked from all eight REQ-39 scenarios. Their dispositions remain `partial`, and their execution statuses remain `not_run`; all 232 frozen scenarios remain `not_run`.
- No tests, Worker/provider activity, formal Mission change operation, or frozen scenario ran.

REQ-39 remains partial pending session-backed qualification and frozen scenarios. Evidence here confirms source/build and local service state only.
