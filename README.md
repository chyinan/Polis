# Polis

**Polis** is an open-source runtime and workbench for building and operating persistent, multi-agent “company” workflows.

It combines a Go + PostgreSQL backend (companies, missions, tasks, workers/sessions, workspaces, artifacts, events, capabilities) with a React workbench UI for creating companies, configuring runtime/capabilities, running missions, and inspecting collaboration, evidence, and operations.

> **Status:** active development. Core local/workbench flows and the capability catalog are implemented; some integrations (external notification delivery, remote/provider-specific production qualification, etc.) depend on your environment and configuration.

## Repository layout

- `cmd/` — CLI and services (e.g. `polis`, `polisd`)
- `internal/` — kernel, control, workbench, provider/runtime, and capability/domain logic
- `db/` — PostgreSQL migrations and SQL/query definitions
- `frontend/` — React + TypeScript workbench UI
- `desktop/` — desktop shell (Tauri) packaging/runtime
- `scripts/` — local tooling and test helpers
- `docs/` — implementation notes and reports
- `spec/` — design/reference material
- `evidence/` — historical verification artifacts (archival)

## Prerequisites

- Go (compatible with `go.mod`)
- PostgreSQL 18 (for runtime-backed usage and DB-backed tests)
- Node.js + npm (for the frontend)

## Getting started

### Backend

```sh
# build
bash scripts/go.sh build ./cmd/...

# test / checks
bash scripts/go.sh test ./...
bash scripts/go.sh vet ./...
```

Notes:
- Many DB-backed tests and integration flows require a **disposable** PostgreSQL database (commonly via `POLIS_TEST_DSN`) and/or helpers in `scripts/`. Do not run tests against production or shared business data.
- Migrations live in `db/migrations/` and are applied via the project’s migration tooling/commands (see `cmd/polis` and `scripts/`).

### Frontend

```sh
cd frontend
npm install
npm run dev
```

Other useful commands:

```sh
npm run typecheck
npm run test
npm run build
```

### Desktop shell

See `desktop/` and `docs/implementation/R0.7_DESKTOP_REPORT.md` for desktop runtime/build notes.

## Configuration & capabilities

- **Runtime/providers:** runtime and provider/model/profile settings (including local deterministic modes and configured HTTP-compatible providers) are managed through backend/workbench APIs. Credentials are handled by the backend and are not exposed to the UI.
- **Capabilities (Skills/MCP):** the capability catalog supports skill/MCP definitions, bindings, probing, descriptor/summary reads, and tool-call execution with request validation and audit receipts, subject to configured transports and kernel/control policies.
- **External side effects:** real outbound integrations (e.g. third-party notifications, remote MCP targets, hosted provider execution) depend on adapters, credentials, and environment-specific setup.

## Documentation

- [`AGENTS.md`](AGENTS.md) — contributor/agent working guide
- [`docs/implementation/NEXT_SLICE.md`](docs/implementation/NEXT_SLICE.md) — current implementation status and suggested next steps
- [`docs/implementation/`](docs/implementation/) — implementation reports and notes
- [`spec/`](spec/) — design/reference snapshots
- [`evidence/`](evidence/) — historical verification artifacts (kept for traceability)

## Contributing

Issues and pull requests are welcome. Please keep changes scoped, follow existing patterns, and include validation (tests/typecheck/build) where practical. For behavior or security-sensitive changes, document impact and configuration clearly.
