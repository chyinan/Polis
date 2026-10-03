# Slice 144 — FT-63 Workbench owner setup and login UI

Date: 2026-10-03

## Delivered

- Added a Group navigation entry for Installation Accounts and a page that checks owner-session status, supports local first-owner enrollment, owner login/logout, displays session expiry, and reads the observed ProviderAccount projection.
- The page offers bootstrap only when the loopback-only setup-status endpoint reports no owner. It accepts the one-time code issued by `polis owner-bootstrap` and validates the 14–1024 UTF-8 byte password policy before sending. HTTPS remote Workbench offers owner login only; first-owner enrollment remains local.
- The account table displays only provider class, account-locator fingerprint, company/session counts and observation times. The page states that the account locator is not billing-scope proof and cannot mutate budgets or company data.
- Credentialed requests use the root-readable CSRF cookie for logout. The local setup-status GET accepts an absent Origin only for a loopback peer, covering same-origin browser reads; bootstrap still requires both an allowed Origin and loopback peer.
- Existing shared desktop-token access remains separate from owner-session status.

## Verification

| Check | Result |
| --- | --- |
| `go test ./internal/desktop ./internal/installationauth ./internal/workbench` | PASS |
| `go build ./cmd/polis` | PASS |
| `sha256sum -c db/migration_hashes.sha256` from `db/migrations` | PASS; all 97 migration files match |
| `git diff --check` | PASS |
| `npm run typecheck` | PASS |
| `npm run lint` | PASS |
| `npm run build` | PASS; Vite reports the existing large JavaScript chunk advisory (>500 kB) |
| PostgreSQL migration/runtime | Not run; `POLIS_DSN` is unset and local PostgreSQL is not accepting connections |
| Browser setup/login/CSRF/logout E2E | Not run; no database-backed service is available. Local Tauri WebView cookie behavior also remains unverified. |

The cookie-to-header CSRF flow follows the session-bound pattern described by the [OWASP CSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html). This is implementation evidence only; it does not qualify the browser behavior or a financial budget boundary.
