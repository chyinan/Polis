# Slice 257 — local development runtime refresh

## Runtime

Rebuilt the ignored Termux backend executable from the current `main` checkout with `go build -o .runtime/bin/polis ./cmd/polis`. `go version -m` reports Android/arm64 and VCS revision `8ad6f84bec1b268bf64afd7dae877e540c34887f`, matching the checkout. With the read-only database checks below showing no Companies, Missions, WorkerSessions, or JobRun events, the prior backend was gracefully stopped and restarted through `.runtime/dev-termux.sh backend`. The backend health endpoint returns HTTP 200 and `{"service":"polis_backend","status":"ready","version":"r0.7"}`. The existing Vite service at port 4173 also returns HTTP 200. PostgreSQL remained running.

## Database observation

A read-only PostgreSQL 18.6 transaction against `polis_r0_termux` confirmed the latest applied Goose migration version is Schema 108. It returned zero Companies, zero Missions, and zero WorkerSessions; `job_run_events` has no rows. The transaction was rolled back. No Worker was created, started, or used.

## Verification limits

- No source changes, migrations, tests, provider operations, or frozen scenarios ran in this slice.
- All 232 frozen scenarios remain `not_run`, and the 17 open software requirements remain open.
- This local health check does not qualify Windows/Linux/Desktop hosts, browser behavior, provider accounts, billing, or R1–R3 scenarios.
- `git diff --check` passed for the handoff update.
