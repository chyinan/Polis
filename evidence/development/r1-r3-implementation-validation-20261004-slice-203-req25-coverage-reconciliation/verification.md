# Slice 203 — REQ-25 owner authorization coverage reconciliation

## Audit

Read the current owner-auth implementation and reviewed the existing Slice 143, 144, 146, and 160 evidence. The previous live REQ-25 coverage row incorrectly described durable sessions, login/logout, CSRF enforcement, and the owner UI as missing.

Current source provides terminal-issued one-time owner bootstrap with Argon2id password storage and rate limits; Schema 97 stores only token/CSRF digests, fixed-expiry sessions and one-time revocation; owner login sets `HttpOnly` and `SameSite=Strict` cookies, adds `Secure` in configured remote HTTPS mode, and gates unsafe cookie-authenticated requests with session-bound double-submit CSRF. `InstallationOwnerPage` provides bootstrap, login/logout, expiry, and read-only account observation. Slice 146 records the flow against disposable PostgreSQL using HTTP clients, including missing-CSRF rejection, logout, and old-session revocation. Recovery revokes active owner sessions as recorded in Slice 160.

## Remaining limits

No owner password was created in the local development database. Slice 146 did not use a real browser; local browser/Tauri WebView cookie behavior and remote-origin qualification remain open. Financial settlement and cross-Company owner mutations remain gated on explicit billing-scope and authorization prerequisites. No auth command, database mutation, owner login, or browser session was performed during this source audit.

## Verification

- Source inspection only; no tests or runtime operation were run.
- `git diff --check` passed.
