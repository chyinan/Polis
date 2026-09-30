# R0.6 E checkpoint — Collaboration UX

> Date: 2026-09-21  
> Status: PASSED targeted baseline

## Delivered

- Added a scoped collaboration read projection over existing `messages`, `obligations`, `contract_revisions` and Artifact evidence references.
- Projection preserves sender, receiver, Mission/Task, content, message kind/delivery state, contract revision, task revision, obligation state and applied evidence independently.
- Added `/api/workbench/companies/:companyId/collaboration` and a real Collaboration Inbox page in the existing frontend navigation.
- Empty, loading and error states are explicit; no new chat product or synthetic messages were introduced.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/kernel ./internal/control ./internal/workbench ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` — 11 tests passed.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-e-pg` — migrations through schema 10 applied; `TestPostgresCollaborationProjectionPreservesMessageAndObligation` passed with a real peer Message and pending Obligation.
- The disposable test caught and fixed nullable `contract_revision_id` scanning before the final pass.

## Known boundaries

- Collaboration is read-only in this slice; human instructions use the separate operator-instruction command and are not misrepresented as peer messages.
- Full message pagination/detail history remains bounded to the current company projection and is not a new scheduler or chat backend.
