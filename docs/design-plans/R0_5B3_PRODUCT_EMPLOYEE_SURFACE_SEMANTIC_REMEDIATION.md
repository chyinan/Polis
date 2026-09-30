# R0.5B3 — Product Employee Surface Semantic Remediation

## Objective and boundary

Replace the real product worker's frozen formatter-oriented tool contract and nonempty-workspace checker with generic Task-scoped workspace and validation semantics. Keep `codex.Tools()` and the R0.3A peer registries unchanged; only `provider.ProductToolSurface()` switches to the new product registry. B3 runs the real `WorkerAdapter` with a local fake transport and a disposable PostgreSQL 18 database. No live provider reservation, provider egress, Medium/High model turn, business account, or external service is in scope.

The existing workspace is one CAS-protected content blob, not a filesystem. Product tools must describe and preserve that abstraction; no path/file-tree API is introduced.

## Accepted domain contract

Mission creation may carry a public `text-acceptance@1` contract containing 1–8 bounded required-text criteria. Only `{{mission_id}}` and `{{task_id}}` are supported placeholders. When Start prepares the product Task, the control plane resolves those identities and inserts an immutable `task_validation_bindings` row with the Task/Mission IDs, acceptance revision, runner kind/revision, resolved contract and configuration digest.

The employee reads this same binding through `work_current` / `context_read`; the model cannot select the checker. `text_contains_all@1` returns structured `PASS`, `FAIL`, `VALIDATION_NOT_CONFIGURED`, `VALIDATOR_UNAVAILABLE`, or `INFRASTRUCTURE_ERROR` results. Missing validation permits exploration and progress checkpoints only; a qualified checkpoint or Artifact submission is denied. There is no nonempty-content pass path.

Workspace mutation uses both expected digest and expected monotonically increasing revision. Product writes, checks and checkpoints additionally require an active WorkerSession on a `working` Task in an `active` Mission; once publication changes the Task to `candidate`, even the still-active session is fenced. A check receipt binds Task, Mission, session, epoch, binding digest, workspace digest and revision. Qualified checkpoint validation checks that tuple transactionally. Product Artifact publication rechecks the current checkpoint and all referenced passing receipts in both staging and publication transactions, then stores an immutable artifact-qualification link.

## Product tool contract

The product registry keeps the current seven capability names, but count is not a compatibility objective. All descriptions and schemas are task-generic:

| Capability | Input | Authority / outcome |
| --- | --- | --- |
| `work_current`, `context_read` | none | Session-bound Handover; includes the public Task validation binding |
| `workspace_read` | none | Current authorized workspace content, digest and revision |
| `workspace_replace` | expected digest, expected revision, complete content | Bound employee/session writer; stale digest or revision conflicts; receipt includes new revision |
| `workspace_check` | none | Dispatch by authoritative TaskValidationBinding; structured public result and persisted receipt |
| `work_checkpoint` | typed progress/qualified checkpoint | Product qualified path requires a passing check for the current binding/session/workspace revision |
| `artifact_submit` | none | Candidate publication requires and revalidates current qualified evidence |

`EmployeeTools` retains its legacy probe checker path behind the existing non-product construction. The real product adapter no longer accepts an injected `OfflineProductCheckRunner`.

## Implementation sequence and evidence

1. Add the pure typed binding and validator registry with positive, negative, missing-binding, unavailable and integrity tests.
2. Add the additive schema 7 migration, public Mission contract persistence, immutable Task binding and read projection.
3. Register the generic product tool surface; implement digest+revision CAS, current-binding check receipts, qualified checkpoint enforcement and transactional Artifact finalization. Leave peer-specific paths intact.
4. Expose and display the public contract in the existing frontend Mission form/snapshot.
5. Validate with a disposable PostgreSQL 18 integration suite and browser flow through the existing frontend → Go command API → control plane → real WorkerAdapter → fake transport. Persist B3-only evidence; do not edit R0.5A/B1/B2 acceptance records.
6. Run complete offline Go, race, vet, Linux/Windows builds, frontend test/typecheck/lint/build, PostgreSQL integration, browser E2E, syntax and diff checks. A passing result makes the new surface eligible for a separately authorized qualification only; it does not start live qualification or R0.5B smoke.

## Definition of done

- Public Mission acceptance criteria are persisted and visible; task-scoped binding is frozen at Start.
- The current product surface has no formatter, probe, fixture, filename, or hidden-checker semantics.
- Workspace CAS requires digest and revision; stale writers are rejected.
- Check, checkpoint and Artifact receipts agree on Task binding, session/epoch and current workspace version.
- A missing binding reports `VALIDATION_NOT_CONFIGURED` and cannot create qualified evidence or an Artifact.
- Offline real-adapter/browser E2E passes with zero provider egress and zero live Medium/High turns; all workers are stopped.
- Historical acceptance evidence remains unchanged, and no live canary or business Mission is started.
