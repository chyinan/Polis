# Slice 72 verification — Linux GitHub Secret Service credentials

Date: 2026-09-29  
Schema: 50 (no migration)  
Status: Linux credential adapter implementation and mock-runner tests passed; host Secret Service and GitHub qualification remain `not_run`.

## Implemented

- Linux `NewProtectedGitHubCredentialStore` returns a store only when `secret-tool` is on PATH and `DBUS_SESSION_BUS_ADDRESS` is present. The provider remains independently gated by `POLIS_GITHUB_READONLY_ENABLED=1`.
- The store uses the current user's Secret Service keyring via fixed `service=polis-github-readonly` and validated `account=<credentialRef>` attributes. It sends token bytes on stdin, not argv or PostgreSQL.
- The subprocess has a five-second deadline, receives an environment allowlist for the session bus/runtime, discards stderr, caps lookup output, and maps operational errors to the existing redacted unavailable error.
- Non-Linux/non-Windows builds keep the fail-closed unavailable adapter. Windows DPAPI storage is unchanged.

## Verification

- Linux adapter tests use an injected runner and cover store/read/delete calls, token argument separation, bounded output, environment filtering, invalid input and redacted operational errors.
- `rtk bash scripts/go.sh test ./internal/feedback/github -count=1` — passed.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- Linux/Windows amd64 command build and Windows amd64 GitHub package test-binary cross-compilation — passed.
- `rtk git diff --check` — passed.
- Independent static code review found no outstanding Critical, Important or Minor findings.
- No Secret Service D-Bus call, keyring item, credential or external GitHub request was made.

## Remaining R2 gaps

Automatic poll scheduling, company-scoped feedback backlog state/terminal archive, Linux keyring host qualification, and live repository permission qualification remain open.
