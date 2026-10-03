# Slice 142 — FT-63 first-owner bootstrap

Date: 2026-10-03

## Delivered

- Schema 96 adds the singleton installation-owner password row, one active bootstrap record, persistent bootstrap failure state and append-only authentication events.
- `polis owner-bootstrap` requires `POLIS_DSN`, a Polis database name and a non-superuser database role. It refuses redirected output and prints a `crypto/rand.Text` one-time code only to an interactive terminal. The raw code is never written to the database or logs; only its SHA-256 digest is stored, and it expires after 10 minutes. An unexpired code cannot be silently replaced.
- `/api/installation/owner/status` and `/api/installation/owner/bootstrap` require a loopback peer and a non-empty locally allowed Origin. Bootstrap accepts a bounded JSON POST, enforces a 14–1024-byte password, limits five invalid attempts per bootstrap window and blocks for 15 minutes, then consumes the code and creates the one owner in a transaction. The password is stored using Slice 141's fixed Argon2id profile.
- The setup routes do not require `POLIS_DESKTOP_SESSION_TOKEN`; their independent local peer and Origin checks prevent remote first-visitor registration. The shared desktop token remains in place for existing management routes.
- No browser owner login, revocable session, cookie, CSRF protection or frontend setup/login flow is present yet. FT-63 is therefore still incomplete.

## Verification

| Check | Result |
| --- | --- |
| `go test ./internal/desktop ./internal/installationauth` | PASS |
| `go build ./cmd/polis` | PASS |
| `sha256sum -c db/migration_hashes.sha256` from `db/migrations` | PASS; all 96 migrations match |
| `git diff --check` | PASS |
| PostgreSQL migration/runtime | Not run |
| End-to-end owner setup in a browser | Not run; UI/session flow is not implemented |
