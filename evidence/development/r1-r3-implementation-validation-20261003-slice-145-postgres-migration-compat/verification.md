# Slice 145 — PostgreSQL migration compatibility and local Schema 97

Date: 2026-10-03

## Migration parser findings and recovery

The existing Termux PostgreSQL 18 cluster was stopped at schema 72. Its `polis_r0_termux` database had no Company or Mission rows and was owned by the non-superuser `polis_runtime` role.

The first migration attempt reached version 77, then Goose rejected migration 78 because the historical SQL file omitted `-- +goose Up`. The database was checked: version 77, no `memory_cas_retention_pins` relation, and no migration evidence row for 78. A compatibility reader now adds the missing Goose marker to migrations 78 and 79 only in the byte stream Goose parses. The checked-in SQL bytes, manifest hashes and migration execution evidence inputs remain unchanged.

The next attempt applied through 96, then Goose split the new migration 97 PL/pgSQL trigger function because it lacked `StatementBegin/End`. The database was checked: version 96, no `installation_owner_sessions` relation, and no migration evidence row for 97. Migration 97 now brackets that function with the required directives and its checksum manifest was updated.

The final migration completed at schema 97. Migration execution evidence contains all versions 78–97. The two earlier migration attempts remain recorded as `outcome_unknown` in the local attempt journal; database state was inspected after each failure before retrying.

## Local development runtime

- PostgreSQL 18.6 is running through `.runtime/dev-termux.sh db-start` on the private Termux data directory and local socket.
- The latest `polis` binary was built into the ignored local `.runtime/bin/polis`.
- `.runtime/dev-termux.sh backend` is running on `127.0.0.1:8080`; `.runtime/dev-termux.sh frontend` is running at `http://127.0.0.1:4173/`.
- Backend `/healthz` returns ready, local setup status returns `initialized:false`, and owner session status returns `authenticated:false`. The owner was deliberately left uninitialized because no owner-chosen password was supplied.
- Vite root returned HTTP 200 and the logout CORS preflight returned 204 with credentials and `X-Polis-CSRF-Token` allowed.

## Verification

| Check | Result |
| --- | --- |
| `go test ./db` | PASS, including the legacy Goose-reader normalization test |
| `go build -o .runtime/bin/polis ./cmd/polis` | PASS |
| `sha256sum -c db/migration_hashes.sha256` from `db/migrations` | PASS; all 97 source migrations match |
| `polis migrate` against local Termux database | PASS; schema 97 and migration execution evidence 78–97 recorded |
| Backend health / owner status / owner session / CORS preflight | PASS |
| Owner bootstrap/login/logout browser E2E | Not run; no owner credential was selected and no browser automation is available |
| Tauri WebView cookie behavior | Not verified |

The running local services and PostgreSQL data directory are ignored local runtime state. Stop them with Ctrl-C in each backend/frontend session and `bash .runtime/dev-termux.sh db-stop` when no longer needed.
