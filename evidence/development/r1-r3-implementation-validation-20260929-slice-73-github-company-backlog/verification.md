# Slice 73 verification — company GitHub feedback backlog

Date: 2026-09-29
Schema: 51

## Implemented

- Added the append-only, company-scoped `feedback_backlog_events` ledger. Backlog decisions bind to the exact latest observed issue revision; stale decisions conflict, and a newly observed revision returns to `needs_review`.
- Added an explicit internal backlog status command and Workbench projection/UI. Provider `RemoteState` remains distinct; the receipt says the remote issue was unchanged. The path does not create Tasks, restart missions, invoke a model, or issue GitHub writes.
- Required the desktop session token for the feedback backlog mutation. The regression test first failed with an unauthorized-status mismatch (`204`, expected `401`) and passed after the auth path was gated.
- Aligned the Workbench current-observation sort with the Kernel revision fence: `source_updated_at DESC, observed_at DESC, revision_sha256 DESC`.

## Verification

- `rtk bash scripts/r2-github-feedback-backlog-postgres-test.sh` — passed using a dedicated disposable PostgreSQL 18 cluster through Schema 51. Covers new issue status, stale revision conflict, idempotency, changed-revision review, remote-state separation and company isolation.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed for Linux/amd64.
- `GOOS=windows GOARCH=amd64 rtk bash scripts/go.sh build ./cmd/...` — passed from WSL with only the build environment forwarded.
- `rtk npm test` — 117/117 passed; `rtk npm run typecheck`, `rtk npm run lint`, and `rtk npm run build` — passed. Vite reported the existing large-chunk advisory.
- `rtk proxy powershell -NoProfile -File scripts/test-migration-hash-manifest.ps1` — passed, including rejection of edits to pinned migrations.
- `rtk git diff --check` — passed.
- Tokenless backlog middleware and path classification regression tests passed after the security fix.

No GitHub credential, account, external API, model, or production database was used. Automatic polling and its explicit collection policy remain separate work; live GitHub and Linux Secret Service host qualification remain unavailable/unqualified.
