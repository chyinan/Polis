# R1–R3 implementation validation — slice 9

Date: 2026-09-24

## Scope

Added a Windows current-user DPAPI store for the single read-only GitHub credential reference `default-readonly`, with bounded token/reference validation, redacted load errors, secure-file rotation and deletion. The Workbench Feedback page now offers a password-only store/delete form; the response shape never contains token material and the token is not written to PostgreSQL. `cmd/polis` can create the stored-token source only when `POLIS_GITHUB_READONLY_ENABLED=1`; the default remains off, and storing a token alone issues no network request. Non-Windows builds explicitly report protected storage unavailable.

Also added the tested bounded source snapshot materializer from slice 8. It remains disconnected from Control and cannot start Node/npm or a JobRun.

## Verification

- `bash scripts/go.sh test -count=1 ./...` — passed serially. Database-backed tests without `POLIS_TEST_DSN` used their explicit skip guards.
- `npm test -- --run` — passed, 10 files / 55 tests.
- `npm run typecheck` — passed.
- `npm run lint` — passed with zero warnings.
- `npm run build` — passed. Vite reported the main JavaScript bundle exceeds its 500 KB advisory threshold.
- Linux and Windows amd64 builds of `./cmd/...` — passed.
- Windows amd64 test binaries for `internal/feedback/github` and `internal/environment` — cross-compiled. The GitHub DPAPI round-trip test was not executed on a native Windows host during this slice.
- `git diff --check` — passed.

## Boundaries and remaining work

No real token was stored, no GitHub request or account was used, and the opt-in flag was not enabled. The Windows credential form stores a token but there is no source lifecycle UI yet. Linux protected credentials, automatic polling, backlog routing, Windows AppContainer filesystem/network isolation, Control wiring for materialization, npm execution, actual JobRun/service lifecycle, desktop-authenticated browser download E2E, and all R3 domain qualification evidence remain open.
