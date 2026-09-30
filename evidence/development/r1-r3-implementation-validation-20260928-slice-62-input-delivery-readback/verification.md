# Slice 62 — Task input delivery receipt and Workbench readback

Date: 2026-09-28  
Schema: 47

## Changes

- Added an append-only `not_required` terminal outcome for Tasks with no model-eligible inputs. The prepared and final records commit together; no delivery ID is returned for later completion.
- Added the model payload digest to the Workbench Task input view. Readback reloads eligible CAS blobs, rebuilds the bounded model context and rejects a stored digest that does not match those bytes.
- The Workbench now distinguishes an empty-input receipt from a Task that has not started input delivery, and shows the payload digest beside the frozen manifest digest.
- Updated `kernel.Open` and the Workbench read store to require Schema 47.

## Verification

- `bash scripts/go.sh fmt ./internal/workbench ./internal/kernel ./db` — passed.
- `bash scripts/go.sh test ./internal/kernel ./internal/workbench -count=1` — passed before enabling the disposable PostgreSQL path; database-backed tests were subsequently covered by the dedicated script below.
- `bash scripts/r1-capability-source-postgres-test.sh` — passed on a disposable PostgreSQL 18 database migrated through Schema 47. This includes actual FakeRuntime Worker delivery followed by Workbench readback, plus an empty-input Worker receipt readback.
- Frontend Vitest — 107 tests across 12 files passed.
- TypeScript build check and ESLint (`--max-warnings 0`) — passed.
- Vite production build — passed; it retains the existing bundle-size advisory.
- `scripts/test-migration-hash-manifest.ps1` — passed, including forward-only migration hash handling.

No real model, QQ account, external MCP endpoint, GitHub account, production database or Windows WFP operation was used.
