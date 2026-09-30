# R1–R3 continuation verification — slice 4

Date: 2026-09-24

Workspace: isolated worktree `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`.

## Passed

- Dedicated PostgreSQL 18 database `polis_r0_envjobs_20260924` migrated incrementally from Schema 23 to Schema 24 using `00024_github_feedback_store.sql`.
- `TestGitHubFeedbackRequiresSourceApprovalAndOnlyCompleteScansAdvanceCursor` passed on Schema 24. It checks source registration, denial before permission qualification/approval, complete scan storage, immutable scan/page/observation links, issue revision append, partial-scan retention, and the rule that only a complete scan advances the cursor.
- `TestZIPInputIsDeliveredToWorkerAndReadBackPerFile` passed on the same Schema 24 database.
- Full offline `rtk bash scripts/go.sh test ./... -count=1` passed with database DSNs unset; PostgreSQL tests are exercised separately.
- Linux `rtk bash scripts/go.sh build ./cmd/...` and Windows amd64 cross-build passed.
- GitHub adapter package tests pass entirely against local `httptest` servers; no real GitHub request or credential was used.

## Still incomplete

- The adapter is not invoked by a scheduler or Control route, and credentials are not integrated with protected storage.
- Durable GitHub comment scan/page/observation records, Workbench coverage and issue readback, and Company backlog routing are not implemented.
- No repository permission qualification against a real account was run. No GitHub issue was triaged into a Mission or Task.
