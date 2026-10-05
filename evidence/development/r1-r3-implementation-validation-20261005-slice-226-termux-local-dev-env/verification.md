# Slice 226 — Termux local development environment

## Change

Rebuilt the local CLI with the current repository source and started the Termux PostgreSQL development service. The initial migration attempt refused to proceed because the existing Schema 102 row's recorded migration digest differed from the current checked-in migration. This integrity check was not bypassed. A full custom-format dump (775 KB) is preserved at `.runtime/termux-pg/backups/polis_r0_termux_schema102-pre-reset-20261005.dump`. Read-only preflight confirmed the old database had zero Companies, Missions and WorkerSessions. Its empty schema was reset, and the standard migration command then applied the current chain through Schema 106.

The backend is running at `127.0.0.1:8080`; `GET /healthz` returned `{\"service\":\"polis_backend\",\"status\":\"ready\",\"version\":\"r0.7\"}`. Vite is running at `http://127.0.0.1:4173/`; an HTTP request returned 200. The running services bind to loopback.

## Verification

- `go build -o .runtime/bin/polis ./cmd/polis`: passed.
- Standard `.runtime/dev-termux.sh migrate`: applied through Schema 106.
- Read-only database check: Goose latest version 106, 106 migration-evidence rows, zero Companies, zero Missions, zero WorkerSessions.
- `sha256sum -c ../migration_hashes.sha256` from `db/migrations/`: all 106 entries passed.
- Backend `/healthz`: HTTP 200, status `ready`.
- Frontend Vite root: HTTP 200.
- No tests, Worker/provider activity or frozen scenario ran.

The previous database dump is local and is not part of the Git commit. The development services remain running in the current Termux sessions.
