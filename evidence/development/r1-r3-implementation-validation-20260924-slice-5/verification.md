# R1–R3 continuation verification — slice 5

Date: 2026-09-24

Workspace: isolated worktree `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`.

## Passed

- Dedicated PostgreSQL 18 database `polis_r0_envjobs_20260924` migrated additively to Schema 25 using `00025_github_feedback_comments.sql`.
- `TestGitHubFeedbackRequiresSourceApprovalAndOnlyCompleteScansAdvanceCursor` passed on Schema 25. It covers draft source registration, denial before permission qualification/approval, immutable issue/page/scan links, complete-scan-only cursor movement, partial scan retention, pinned comment observations and update/delete immutability.
- `TestZIPInputIsDeliveredToWorkerAndReadBackPerFile` and the real Workbench multipart ZIP upload/readback test passed on the same Schema 25 database.
- Full offline `rtk bash scripts/go.sh test ./... -count=1` passed with database DSNs unset; selected PostgreSQL tests were run separately on the dedicated DB.
- Linux `rtk bash scripts/go.sh build ./cmd/...` and Windows amd64 cross-build passed.
- GitHub collector tests pass against local `httptest` servers only. No account token or external GitHub API request was used.

## Remaining feedback work

- `Control` does not yet invoke the adapter. No polling scheduler, protected credential setup, Workbench source/coverage view, backlog routing, or task handoff is wired.
- The immutable source/issue/comment ledger exists, but these records have not been used with a live account or qualified as a real feedback workflow.
