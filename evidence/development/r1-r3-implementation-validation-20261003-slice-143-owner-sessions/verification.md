# Slice 143 — FT-63 revocable owner sessions and CSRF boundary

Date: 2026-10-03

## Delivered

- Schema 97 stores only SHA-256 digests for random owner session and CSRF tokens. Session rows have a fixed 12-hour expiry, immutable token facts, and a one-time revocation transition. Login events join the append-only authentication event ledger.
- Password login uses the stored Argon2id verifier, verifies that the owner revision did not change while the hash was checked, records successful and failed attempts, and enforces a persisted five-failure / 15-minute throttle.
- Login sets an HttpOnly, SameSite=Strict session cookie and a readable SameSite=Strict CSRF cookie. Both use Secure when an exact HTTPS remote Workbench origin is configured. Logout revokes the server-side session and clears both cookies.
- Unsafe requests authenticated by an owner cookie require an Origin and a double-submit token in `X-Polis-CSRF-Token`; the token must match the CSRF cookie and its digest must match the active session record. The desktop middleware continues to accept the existing desktop token and now accepts an authenticated owner session on protected Workbench paths.
- The observed installation ProviderAccount read route now accepts owner-session authentication in addition to the existing validated desktop token.
- No cross-Company financial mutation was added. The frontend setup/login/session/logout flow remains open.

## Verification

| Check | Result |
| --- | --- |
| `go test ./internal/installationauth ./internal/desktop ./internal/workbench` | PASS |
| `go build ./cmd/polis` | PASS |
| `sha256sum -c db/migration_hashes.sha256` from `db/migrations` | PASS; all 97 migration files match |
| `git diff --check` | PASS |
| PostgreSQL migration/runtime | Not run; `POLIS_DSN` is unset and local PostgreSQL is not accepting connections |
| Browser owner login/logout and cookie/CSRF E2E | Not run; UI is not wired yet |

CSRF uses a session-bound cookie-to-header design with exact allowed-origin CORS and SameSite cookies. Reference: [OWASP CSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).
