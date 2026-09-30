# R0.5B17 qualified checkpoint read-model projection hardening

Status: PASSED offline/local. No LIVE_6, canary, provider turn, High turn, or multi-agent E2E was run.

## Projection root cause

`READ_QUERY_RELATION_DEFECT` with a missing authoritative checkpoint projection.

The frozen LIVE_5 delivery persisted its product qualification in `task_validation_artifact_qualifications`. The old Workbench read path joined only the legacy `artifact_qualifications` table, and it had no direct `worker_checkpoints` read projection. Consequently the Task and Artifact were visible while `ArtifactSummary.checkpointId` was null; Activity remained visible because it is a separate event projection.

Frozen LIVE_5 authoritative identifiers:

- Company: `r05b-live-5`
- Mission: `81ee562d602a059ed8a4ce2f5be80803`
- compat Task: `7682cd197e8436a763f22ce3198a60ce`
- WorkerSession: `dfc5dd021ec9550ca299903458a63e3b`
- final qualified checkpoint: `e1053ac73bab502e04bfa3bb8b35e39e`
- Artifact: `9ba046913a0d58ca0ad58df867f297b9`
- Artifact/workspace digest: `531f5988cba3b3edb01ad3671a0b4014fdf57df96bebaab9c9e199013d80021b`

The immutable historical `authoritative-final-state.json` records the old failure: Artifact existed but `checkpointId` was null, while `postgres-final-state.json` and `live5-state-query.txt` contain the qualified checkpoint and product qualification relation.

## Corrected read path

PostgreSQL `worker_checkpoints` is now joined to `worker_sessions` and mission-scoped `tasks`. The projection retains stopped sessions and candidate Tasks, selects deterministically by WorkerSession epoch, workspace revision, qualified kind, and stable checkpoint ID, and never uses browser arrival order or Activity events.

The read DTO now exposes `checkpoints[]` with checkpoint, Task, employee/session, kind/state, qualification state, workspace revision/digest, and Artifact relation. Artifact projection coalesces the current `task_validation_artifact_qualifications` relation with the legacy relation for historical compatibility, and exposes its checkpoint/check receipt relation.

The frontend `RealWorkbenchApi` validates and preserves this DTO. Overview, Task detail/jobs/browser, and Employee memory views select the checkpoint by Task relation. “暂无 checkpoint” is retained only when the authoritative DTO has no matching checkpoint.

## Verification

- PostgreSQL regression: `postgres-read-model-test.log` passed. Covers no checkpoint, progress, progress plus newer qualified, candidate plus Artifact, other-Task isolation, and stale/older WorkerSession ordering.
- Frozen-LIVE_5-equivalent backend replay: authoritative checkpoint, read API DTO, Artifact relation, candidate Task, and stopped session all passed.
- Browser E2E: `browser-e2e-result.json` passed. DB/API/browser identities matched for checkpoint and Artifact; candidate Task was visible; Activity exposed the checkpoint event.
- Frontend: 22 Vitest tests passed; typecheck, lint, and build passed.
- Go: `go test ./...`, `go test -race ./...`, `go vet ./...`, Linux build, Windows build, and `git diff --check` passed.

## Historical verdict discipline

LIVE_5 raw verdict remains `FAILED` and its evidence is untouched. The separate post-fix interpretation is:

| Dimension | Result |
| --- | --- |
| LIVE_5 provider execution | PASSED |
| LIVE_5 delivery transaction | PASSED |
| LIVE_5 authoritative backend state | PASSED |
| LIVE_5 original checkpoint projection | FAILED |
| post-fix checkpoint projection replay | PASSED |

## Provider freshness

Offline current-surface checks recomputed the unchanged product identity:

- `polis-product-tool-surface@4`
- manifest: `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`
- schema: 2206 bytes / `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`
- exact execution fingerprint: `e736d8298f0920c6dd81796006b2486881647f0fdbc66bb111bbd2387a1e99ff`
- provider L2 fingerprint: `59a4ac003fb0f49360382a707f7057edc03e0574ba328d0cf3a59eeab2726781`
- launch envelope fingerprint: `dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d`

No provider-visible or execution identity drift was introduced. B14 remains `QUALIFIED_REUSABLE`. Task provider egress for this hardening was `0`; a new live product smoke is eligible only under separate authorization. LIVE_6 is not needed for this read-path fix.
