# Slice299 REQ-02 durable semantic TaskRevision context

Date: 2026-10-07

## Scope

Schema122 adds append-only `task_semantic_revisions` rows bound to the exact
Task, owner-confirmed RoleRevision digest, semantic task type, persisted
TaskKind and content-addressed binding. The product Worker admission path
records the `compat`/Backend binding only when the optional Schema122 table is
present; absence of Schema122 skips only this write while the existing Schema121
fixed-team admission gate remains required. Insert/replay is idempotent and
read-back is compared inside the same transaction.

Rows are explicitly `qualification=unverified`, `requires_human=true` and
`task_revision_unqualified`; this is durable context, not Worker/Provider
execution permission. The migration has immutable/no-truncate protection and
foreign keys and a provenance trigger enforce Task owner/kind and RoleRevision
task_type/TaskKind alignment.

## Verification

Passed:

- `rtk bash scripts/go.sh test ./db`
- `rtk bash scripts/go.sh test ./spec -run TaskRevision -count=1`
- `rtk bash scripts/go.sh test ./internal/kernel -run Worker -count=1`
- `rtk bash scripts/go.sh test ./...`
- `rtk bash scripts/go.sh build ./cmd/...`
- `rtk git diff --check`

No migration was applied, and no database runtime, WorkerSession,
Provider/browser process, external account or frozen scenario was used.

## Remaining boundary

The durable context now exists in source, but the runtime remains unqualified:
Task/PlanRevision approval lifecycle, owner qualification, provider execution,
and full database/runtime evidence remain open.
