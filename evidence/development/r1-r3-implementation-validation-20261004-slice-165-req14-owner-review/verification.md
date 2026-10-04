# Slice 165 verification — REQ-14 owner-reviewed unresolved disposition

Date: 2026-10-04

## Change

Schema 100 adds an immutable installation-owner review table for active capability revocations whose exact revoke-time WorkerSession inventory is incomplete. The only supported disposition is `acknowledged_unresolved`. The Workbench POST route requires a validated installation-owner session and a CSRF token. Inside the Company-guarded write, Kernel rebuilds the current revocation projection and accepts a review only for an active, still-incomplete revocation without an existing review. The immutable record binds the revocation identity, scope, capability revision, owner rationale, request ID and review time.

The projection keeps `sessionInventoryComplete=false` and `quiesced=false` after review. The action does not infer an exact historical session set from usage events, certify that all historical sessions stopped, or invoke any Worker stop. Workbench shows the owner review and retains the warning state.

## Validation

- `go build -o /data/data/com.termux/files/usr/tmp/polis-slice165-linux ./cmd/polis` — passed.
- `GOOS=windows GOARCH=amd64 go build -o /data/data/com.termux/files/usr/tmp/polis-slice165-windows.exe ./cmd/polis` — passed.
- `go build -o .runtime/bin/polis ./cmd/polis` — passed; local development executable refreshed.
- `npm run build` in `frontend/` — passed (`tsc -b` and Vite production build); Vite reported its existing >500 kB chunk advisory.
- `cd db/migrations && sha256sum -c ../migration_hashes.sha256` — all migration hashes passed, including Schema 100.
- `./.runtime/dev-termux.sh db-start` — PostgreSQL was already running.
- `./.runtime/dev-termux.sh migrate` — Schema 100 applied successfully.
- Read-only local database count query returned `100|0|0|0` for migration version, Companies, installation-owner credentials, and WorkerSessions.
- `git diff --check` — passed.

## Not run

No tests were run. No owner review was submitted because the local database has no installation-owner credential or Company. No WorkerSession was present or started/stopped, and no provider, MCP endpoint, browser, or Tauri activity ran. The owner-authenticated route and UI have build/source validation only; live owner-cookie/CSRF qualification remains open.

## Status

REQ-14 remains open. This slice provides a durable and explicit way for an installation owner to acknowledge that a legacy session inventory cannot be proven; it does not repair historical evidence or establish quiescence. Applicable owner action and lifecycle/runtime qualification remain outstanding.
