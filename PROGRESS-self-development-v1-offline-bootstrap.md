# Progress: Polis self-development v1 offline bootstrap

> Created: 2026-10-08 | Status: blocked — isolated execution qualification required

## Goal
Implement and verify an offline Polis development Mission loop using a disposable project and controlled FakeRuntime, while preserving provider egress at zero and avoiding changes to the running Polis, original repository, production data, and historical qualification evidence.

## Acceptance criteria
- Audit existing capabilities against the requested nine areas and classify each READY, PARTIAL, BLOCKED, or MISSING from source and formal implementation evidence.
- Implement the smallest authorized-repository and isolated-candidate workspace execution path by reusing current Company/Mission/Task/WorkerAdapter/Workspace/Artifact boundaries.
- Produce a real offline diff, test result/log, Patch Artifact, and human accept/reject review state.
- Demonstrate original repository and running Polis state remain unchanged; do not claim OS isolation qualification unless existing evidence supports it.
- Record exact blockers and minimum conditions/budget for a separately authorized first real Codex development Mission. Keep REQ-38 external retrieval gates independent.

## Constraints
- Go / PostgreSQL 18 / pgx / sqlc / goose; one principal code writer.
- No real provider Mission, provider egress, automatic merge/push/deploy, ambient Codex credentials, unapproved Skill/MCP execution, or self-repair loop.
- Disposable test projects and dedicated temporary resources only; preserve existing worktree changes and historical evidence.

## Read files
- `AGENTS.md` — user-supplied repository instructions and stage/qualification constraints.
- `C:\Users\chyinan\.codex\RTK.md` — shell commands must be prefixed with `rtk`.
- `C:\Users\chyinan\.agents\skills\focused-problem-solver\SKILL.md` — required progress and summary workflow.
- `C:\Users\chyinan\.agents\skills\investigating-a-codebase\SKILL.md` — investigation workflow.
- `C:\Users\chyinan\.agents\skills\coding-effectively\SKILL.md` — required implementation principles.
- `C:\Users\chyinan\.agents\skills\functional-core-imperative-shell\SKILL.md` — file classification and pure-core/I/O-shell separation.
- `C:\Users\chyinan\.agents\skills\test-driven-development\SKILL.md` — red/green implementation workflow.
- `C:\Users\chyinan\.agents\skills\verification-before-completion\SKILL.md` — evidence required before status claims.
- `C:\Users\chyinan\.agents\skills\using-plan-and-execute\SKILL.md` — skill discovery, announcements, and task tracking workflow.

## Current progress
The read-only capability audit is complete and recorded in `evidence/development/polis-self-development-v1-offline-bootstrap/assessment.md`; the codebase summary is in `summary/polis-self-development-v1-offline-bootstrap.md`. Generic Mission creation and candidate Artifact preview are reusable, but there is no development Mission/repository binding, code-tree workspace, project command runner, Skill/MCP execution, Patch Artifact workflow, or human accept/reject action. Existing fixed bwrap verification is not general hostile-worker qualification; no disposable target code was executed. Overall offline bootstrap is BLOCKED at the isolated workspace/build-test gate.

## Next
Resume Phase B only after a qualifying writable-workspace execution boundary is available. Any real Codex Mission still requires separate explicit authorization.

## Findings
- No real provider execution is authorized; user explicitly requires `provider_egress = 0`.
- Host directory access/execution qualification is a hard gate; a FakeRuntime can verify application flow but cannot establish OS sandbox qualification.
- `spec/design-v0.4.5/product/RELEASE_SCOPE.json` currently records REQ-38 as `not_implemented` / `not_run`; `tests/traceability.json` maps REQ-38 to WF-16 through WF-20. `TEAM_COVERAGE.json` is a fixed-team design draft (`execution_enabled=false`, coverage qualifications `unverified`), not runtime execution evidence.
- `internal/runner/verifier.go` contains a bwrap-based, fixed Go-test verifier that read-only binds the candidate tree and disables network; this may support controlled tests but does not provide a writable development workspace or by itself qualify hostile-worker containment.
- Existing local Codex production launch binds a dedicated home and empty `/work`; it has no authorized repository workspace mount. The current Windows process implementation uses a process handle but has no child-tree containment in that file.
- Read-only investigator confirmed: Mission API persists only title/goal/acceptance; `worker_workspaces` is a single `Content` blob; current Workbench task controls include create/start/cancel and Artifact read/preview, with no user accept/reject operation. Catalog APIs list/import Skills and register MCPs only.
- Existing artifact publication/check receipts are reusable, but current Artifact rows are bounded to 1–4096 bytes and do not represent repository diffs or project test-log bundles.
- Official records: `RELEASE_SCOPE.json` marks REQ-38 planned/not implemented/not run; `traceability.json` maps REQ-38 to WF-16–WF-20. `TEAM_COVERAGE.json` is a design draft with execution disabled and unverified qualifications.
- Existing `AGENTS.md`, `NEXT_SLICE.md`, and `PROGRESS.md` contain stale/conflicting handoff text in the dirty worktree; this task follows the explicit user-provided scope and the R0.8 report, without rewriting those pre-existing edits.
- Offline self-development E2E was not run. No project clone, Mission, WorkerSession, target command, or provider process was created; provider egress remains zero.
