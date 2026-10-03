# Slice 148 — REQ-15 WorkerSession-bound memory correction commands

Date: 2026-10-03

## Delivered

- Added Schema 98 compound foreign keys that retain the exact proposer/reviewer WorkerSession and Task identity alongside the persisted Employee. Legacy correction rows stay nullable because their original session provenance cannot be reconstructed.
- Added Kernel correction proposal and review wrappers that accept a WorkerSession ID, resolve Employee and Task from persisted rows, require the current active provider session and recheck the working Task owner inside the Company-locked write. The session and Task are part of command idempotency fingerprints; a client-supplied Employee ID is never accepted.
- Added no-store Workbench proposal/review routes and real-mode forms. Proposal candidates come from active company sessions. Review candidates are limited to active `emp-planning` / `emp-review` sessions and exclude the proposer. The existing Kernel policy still checks independent authorship, memory read scope, role and sensitivity.
- The queue displays session and Task provenance but continues to omit memory content and free-text reasons. Approval requires explicit acknowledgement that it creates a new verified revision and invalidates dependent work.
- Fixture commands remain unavailable and return the existing simulated-mode boundary.

## Verification

| Check | Result |
| --- | --- |
| `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis` | PASS |
| `go build -o .runtime/bin/polis ./cmd/polis` | PASS |
| `npm run typecheck` | PASS |
| `npm run lint` | PASS |
| `npm run build` | PASS; Vite reports the existing >500 kB JavaScript chunk advisory |
| Migration checksum manifest | PASS; all 98 SQL files match |
| Local `.runtime/dev-termux.sh migrate` | PASS; PostgreSQL 18.6 advanced from Schema 97 to 98 |
| Migration execution evidence | `98:applied_during_this_run` |
| Main development database scope | PASS; zero Companies, WorkerSessions, correction requests and review events |
| Local backend `/healthz` and Vite `/` | PASS; both HTTP 200 after restarting the backend from the new binary |
| `git diff --check` | PASS |
| Tests and correction command E2E | Not run; the project instruction for this work prohibits adding/running tests unless requested, and the main development database has no owner, Company or active WorkerSession |

This advances the implementation only. FT-37/40/41 and FT-74 scenario execution remains `not_run`; PostgreSQL Desktop-restore behavior and browser/Tauri owner-session qualification remain unqualified. The owner password was not initialized.
