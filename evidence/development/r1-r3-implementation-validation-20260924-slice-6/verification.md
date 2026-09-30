# R1–R3 implementation validation — slice 6

Date: 2026-09-24

## Scope

Added a company-scoped `GET /feedback` Workbench projection for persisted GitHub source status, issue revisions and comments. The React Feedback page now displays coverage and bounded issue/comment text as read-only untrusted content. SQL limits output by UTF-8 worst-case character bounds; the browser DTO validator independently rejects cross-company data, excessive record counts and oversized text.

## Verification

- `bash scripts/go.sh test -count=1 ./internal/workbench` — passed.
- Dedicated PostgreSQL test `TestPostgresReadStoreProjectsScopedBoundedGitHubFeedback` against `polis_r0_envjobs_20260924`, schema 25 — passed. It verifies source approval/permission/coverage projection, issue and comment readback, company isolation, and title/body/comment byte caps with multibyte UTF-8 fixtures. An initial run exposed the SQL character-vs-byte truncation defect; the fixed implementation passed.
- `bash scripts/go.sh test -count=1 ./...` — passed. Database-backed tests without `POLIS_TEST_DSN` were skipped by their explicit guard.
- `GOOS=linux GOARCH=amd64 bash scripts/go.sh build ./cmd/...` — passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` — passed.
- `npm test -- --run` — passed, 10 files / 52 tests.
- `npm run typecheck` — passed.
- `npm run lint` — passed with zero warnings.
- `npm run build` — passed. Vite reported the existing main JavaScript bundle exceeds its 500 KB advisory threshold.

## Boundaries

No GitHub account, external GitHub request, token, polling scheduler, backlog route, QQ send, model turn, MCP call, project process, npm install, deployment or publication was used. Control polling, backlog routing and live GitHub permission qualification remain open.
