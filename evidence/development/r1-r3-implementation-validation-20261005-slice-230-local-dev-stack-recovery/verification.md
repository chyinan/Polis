# Slice 230 — local development stack recovery

## Change

Recovered the Termux development stack after its PostgreSQL and application services had stopped. The checked-in backend binary started cleanly against the existing Schema 108 database. PostgreSQL, backend, and Vite are listening on local interfaces; no database reset or migration was needed.

## Verification

- `bash .runtime/dev-termux.sh db-start`: PostgreSQL started on the configured local socket and port 55432.
- `.runtime/dev-termux.sh migrate`: no migrations to run; current version 108.
- Read-only PostgreSQL query: 108 is the latest applied schema; 0 Companies, 0 Missions, and 0 WorkerSessions.
- `GET http://127.0.0.1:8080/healthz`: HTTP 200, `status=ready`, backend version `r0.7`.
- `GET http://127.0.0.1:4173/`: HTTP 200 from Vite.
- No tests, Worker/provider activity, formal Mission operation, or frozen scenario ran.

The local development stack is running in the current Termux session. This records local availability only; it does not change R1–R3 qualification or scenario dispositions. All 232 frozen scenarios remain `not_run`.
