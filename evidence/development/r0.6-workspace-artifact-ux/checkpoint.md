# R0.6 F checkpoint — Workspace / Checkpoint / Artifact UX

> Date: 2026-09-21  
> Status: PASSED targeted baseline

## Delivered

- Added authorized task workspace preview and Artifact detail read APIs.
- Workspace presentation reflects the current runtime’s single CAS blob honestly as `workspace.txt`, including digest, revision, changed files and checkpoint history; it does not expose arbitrary host filesystem paths.
- Artifact detail validates the CAS digest before returning content and reports content availability separately from Artifact verdict/state.
- Added Task jobs/browser panels that load workspace and Artifact detail through the existing RealWorkbenchApi path with loading/error/unavailable states.
- Added path/scope validation and bounded content preview handling.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/workbench ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` — 11 tests passed.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-f-pg` — schema 10 applied; `TestPostgresWorkspacePreviewReadsAuthorizedCASBlob` passed with a real product workspace blob; cluster stopped and removed.

## Known boundaries

- Current runtime stores one workspace blob, so the UI does not invent a multi-file tree or synthetic diff.
- Artifact materialization/download is not enabled; content preview remains backend-authorized and read-only.
