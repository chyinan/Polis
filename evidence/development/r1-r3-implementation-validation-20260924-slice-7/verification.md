# R1–R3 implementation validation — slice 7

Date: 2026-09-24

## Scope

Added Control commands for explicitly registering a fixed GitHub repository, probing read permission, recording manual approval/pause/revocation, and starting a bounded one-shot issue/comment poll. Polling requires current approved + verified state before provider access; it is capped to a 90-second context, three issue pages/300 issues, and five recent issue comment contexts with one page/100 comments each. Injected credentials are used only by the fixed-host GET-only client; `cmd/polis` does not configure a token source and therefore remains externally disabled. Probe/poll retries with the same request identity return durable evidence rather than repeating provider reads.

The existing Feedback page now reads back company-scoped source/coverage/issue/comment state. SQL and React enforce bounded UTF-8 projection sizes; the dedicated fixture exercises CJK issue titles, bodies and comments and verifies the partial-context indicator.

## Verification

- Dedicated PostgreSQL tests were run **sequentially** against `polis_r0_envjobs_20260924`, schema 25:
  - `TestGitHubFeedbackControlProbesApprovesPollsAndDoesNotCallUnapprovedSource` — passed. It verifies default token-source-off behavior, no provider call before approval, permission probe, manual approval, bounded fake-provider issue/comment persistence, no extra call on probe/poll replay, and denial after source revocation.
  - `TestPostgresReadStoreProjectsScopedBoundedGitHubFeedback` — passed. It verifies scoped source/issue/comment readback, approval/coverage state and UTF-8 byte limits.
- An earlier attempt launched both database tests concurrently against that same dedicated database; the Workbench test failed during `kernel.Open` with `POLICY_DENIED`. Running the tests sequentially passed without code changes.
- `bash scripts/go.sh test -count=1 ./...` — passed when run serially; database-backed cases without `POLIS_TEST_DSN` used their explicit skip guards.
- The timing-sensitive `TestR03AT21StopDuringPostOutputReconnectRequiresTerminationConfirmation` passed 10 isolated repetitions. One earlier full-suite run concurrent with two cross-builds and a database test missed the test's `TurnRecovered` lifecycle event; its fixture waits a fixed 30 ms before stopping. The serial full-suite rerun passed without modifying that historical test.
- `GOOS=linux GOARCH=amd64 bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` — passed.
- Frontend: 10 Vitest files / 52 tests, typecheck, lint and production build passed. Vite reported the existing main JavaScript bundle exceeds its 500 KB advisory threshold.

## Boundaries and remaining work

No GitHub account, token, external request, automatic polling schedule, company backlog mutation, QQ send, model turn, MCP call, npm install, project process, deployment or publication was used. Protected GitHub token storage/UI, automatic polling, backlog routing, source lifecycle UI and live permission qualification remain open. R2 is still not release-qualified; R3 profiles remain `not_run` and execution-disabled.
