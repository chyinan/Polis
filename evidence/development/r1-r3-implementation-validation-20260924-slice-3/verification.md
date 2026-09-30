# R1–R3 continuation verification — slice 3

Date: 2026-09-24

Workspace: isolated worktree `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`.

## Passed

- `rtk bash scripts/go.sh test ./internal/feedback/github -count=1` passed.
- Full offline `rtk bash scripts/go.sh test ./... -count=1` passed with `POLIS_DSN` and `POLIS_TEST_DSN` unset; database integration tests were skipped in that command.
- Linux `rtk bash scripts/go.sh build ./cmd/...` and Windows amd64 cross-build passed.
- Local fake-server tests cover exact repository ID/name verification before polling, repository-scoped token-source selection, paginated Issues scans, PR exclusion, overlap dedupe, page/host/filter/repository-ID pagination-link rejection, incomplete-page watermark protection, bounded issue title/body with original body digest, issue detail recheck before comments, PR comment rejection, comment truncation/revision, unauthorized responses and rate-limit backoff classification.
- Official API behavior was checked against GitHub REST documentation: Issues may include pull requests, `Link` pagination controls next-page URLs, page size is capped at 100, and rate-limit headers/retry behavior must be respected. Relevant sources: `https://docs.github.com/en/rest/issues/issues`, `https://docs.github.com/en/rest/issues/comments`, `https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api`, `https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api`.

## Not qualified or not integrated

- No GitHub account, token, repository access, or external HTTP request was used; tests use local `httptest` servers.
- The adapter is not yet connected to PostgreSQL `FeedbackSourceBinding`/scan/page/observation storage, overlap cursor advancement, scheduling, credentials UI, Company backlog, or Workbench readback.
- This slice does not claim R2 feedback qualification, coverage completeness, or user/business outcomes.
