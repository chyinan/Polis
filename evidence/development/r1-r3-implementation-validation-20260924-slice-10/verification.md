# R1–R3 implementation validation — slice 10

Date: 2026-09-24

## Scope

The Feedback page now provides company-scoped controls to register a fixed GitHub repository, explicitly probe read permission, record manual approval/pause/revocation, and start a bounded one-shot issue/comment poll. Provider calls are made only after the user presses the probe or poll button; page load reads only persisted Workbench data. Poll remains server-gated on approved + verified source status. The UI also provides the DPAPI credential store/delete form introduced in slice 9.

## Verification

- `npm test -- --run` — passed, 10 files / 57 tests.
- `npm run typecheck` — passed.
- `npm run lint` — passed with zero warnings.
- `npm run build` — passed. Vite reported the main JavaScript bundle exceeds its 500 KB advisory threshold.
- `bash scripts/go.sh test -count=1 ./internal/feedback/github ./internal/control ./internal/workbench ./cmd/polis` — passed.
- Cross-compiled Windows amd64 test binaries for `internal/environment` and `internal/feedback/github` were executed on the local Windows host and passed. This exercised Windows snapshot path handling and DPAPI store/read/rotation/delete with a fake token in a temporary directory; the GitHub client tests used local HTTP fixtures only.
- The full Go suite and Linux/Windows builds passed in slice 9 immediately before this frontend-only source-lifecycle change.
- `git diff --check` — passed.

## Boundaries

No GitHub account/token was used and no GitHub request was made. No Workbench probe or poll button was invoked against a live backend. `POLIS_GITHUB_READONLY_ENABLED` remains unset; the app does not start an automatic poller. Non-Windows protected token storage, automatic scheduling, backlog routing and live permission qualification remain open.
