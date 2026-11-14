# Polis — Contributor & Agent Guide

Polis is an open-source runtime and workbench for running persistent, multi-agent “company” workflows. The backend is authoritative for state and execution; the frontend is the presentation layer.

## Stack
- **Backend:** Go, PostgreSQL 18, pgx, sqlc, goose (migrations in `db/migrations/`)
- **Frontend:** React + TypeScript + Vite (`frontend/`)
- **Key dirs:** `cmd/` (binaries), `internal/` (kernel/control/provider/workbench/etc.), `db/` (migrations, queries), `frontend/`, `scripts/`, `docs/`, `spec/`

## Architecture notes
- **State authority:** PostgreSQL is the source of truth. `frontend/` is presentation-only and should consume backend/workbench APIs (default production path: `RealWorkbenchApi`); it must not read the database or provider credentials directly.
- **Schema:** migrations are sequential in `db/migrations/`. The kernel enforces a schema version gate at startup—new schema-dependent features require a migration and gate update.
- **Kernel invariants:** preserve transactional boundaries, writer/session fencing, CAS/workspace digest-revision checks, idempotent command handling, and safe credential handling (secrets must not be returned to the UI or logged in receipts).
- **Capabilities/MCP:** skills and MCP definitions can be registered/bound in the catalog. Tool probing/description/listing are read-oriented; `tools.call` executes via configured transports with request validation, digests, and audit receipts. Keep changes consistent with the existing kernel/control/workbench contracts.

## Local development
Common commands (run from repo root; many scripts target a Linux/WSL environment):

```sh
# Go
bash scripts/go.sh test ./...
bash scripts/go.sh build ./cmd/...
bash scripts/go.sh vet ./...

# Frontend
cd frontend
npm install
npm run dev
npm run typecheck
npm run test
npm run build
```

- Use disposable/local databases for development and tests (e.g. dedicated `polis_r0_*` databases). Do not point tests at production or shared business data.
- Integration tests that require PostgreSQL may be gated by environment variables (e.g. `POLIS_TEST_DSN`) and/or helper scripts in `scripts/`.

## Working conventions
- Keep changes scoped and consistent with existing patterns; prefer fixing root causes over adding parallel paths.
- Validate behavior when possible (targeted tests/typecheck/build) and report what was or wasn’t verified.
- Treat `evidence/` and historical implementation reports as archival context; avoid rewriting or deleting historical records unless explicitly requested.
- Be explicit about security-sensitive changes (credential storage, external network/tool execution, auth, and data access). Favor safe defaults.
