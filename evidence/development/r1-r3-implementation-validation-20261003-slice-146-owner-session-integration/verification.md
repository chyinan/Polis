# Slice 146 — FT-63 owner-session API integration

Date: 2026-10-03

## Delivered

- The Installation Accounts screen is reachable from the browser real-mode setup screen even when `VITE_WORKBENCH_COMPANY_ID` is unset. A Company record is not required to inspect or configure installation-owner state.
- The HTTP owner flow was exercised against a disposable PostgreSQL 18 database migrated to Schema 97. The test issued a temporary terminal bootstrap code and used a temporary test-only password. It did not create an owner credential in the main development database.

## HTTP integration results

| Operation | Result |
| --- | --- |
| `POST /api/installation/owner/bootstrap` from allowed local Origin | 201 |
| `POST /api/installation/owner/login` | 200; session and CSRF cookies issued |
| `GET /api/installation/owner/session` with session cookie | `authenticated: true` |
| `GET /api/workbench/installation/provider-accounts` with owner session | 200 |
| `POST /api/installation/owner/logout` without CSRF header | 403 |
| Logout with bound CSRF cookie/header | 200; session revoked |
| Old session cookie after logout | `authenticated: false` |

The disposable database was dropped after the run. Its local migration/build journal remains under the ignored `.runtime/termux-pg/owner-smoke-*` directory; it contains no raw password or bootstrap code. The main `polis_r0_termux` database is at Schema 97 and still has no owner credential or Company rows.

## Verification and limits

| Check | Result |
| --- | --- |
| `npm run typecheck` | PASS |
| `npm run lint` | PASS |
| `npm run build` | PASS; Vite reports the >500 kB JavaScript chunk advisory |
| Backend/bootstrap/session/CSRF/logout integration | PASS against a disposable PostgreSQL 18 database |
| Real browser UI E2E | Not run; no browser automation is available in this environment |
| Tauri WebView cookie behavior | Not run |
| External provider, financial or cross-Company mutation | Not performed |
