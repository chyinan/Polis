# Polis cloud Codex handoff

Updated: 2026-10-01

## Objective and boundaries

Continue the user-approved Polis R1–R3 implementation from the frozen v0.4.5 design pack and the live coverage ledger at `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`. Work toward the full approved plan; do not redefine completion around the slices already present. Keep the work finite and tied to the existing REQ/FT/NT/CAP/UI/WF/PP traceability.

Explicit exclusions remain office/3D, dynamic hiring/firing, arbitrary MCP compatibility, and a plugin marketplace. Do not run real employee model turns, real QQ sends, external MCP/GitHub actions, business-account flows, or production actions without separate authorization. Preserve historical R0 evidence and the original design snapshot.

## Source of truth

- GitHub repository: `https://github.com/chyinan/Polis`
- Branch: `main`
- Current code checkout used for this implementation: `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`
- Slice101 feature commit: `998f425` (`feat: add fake-only product direct messaging`), followed by review-fix commits `aa0a53e` and `6df7d54`. They add ordered Task row locks, an active-Mission `FOR SHARE` lock, and explicit rejection of an omitted `actionable` field. Read-only re-review found zero remaining findings.
- An older local checkout exists at `D:\Programs\Polis` on `master` with unrelated uncommitted/untracked files. Do not use that checkout as the current source of truth or copy its working-tree contents into `main`.

Clone/pull `main` before continuing. Read this file, `AGENTS.md`, `docs/implementation/NEXT_SLICE.md`, `docs/implementation/PROGRESS.md`, and `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md` first. The coverage ledger is authoritative for the finite remaining scope.

## Just-closed slice

Slice101 connects direct messaging to the generic product Worker adapter through the separately versioned 12-tool `polis-product-tool-surface@7`. It provides bounded same-Mission target discovery, direct send, ordered inbox, acknowledgement, application evidence, and candidate-Artifact resolution. The Kernel locks the Mission row `FOR SHARE`, then locks source and target Tasks in stable ID order, then checks the current WorkerSession Task, explicit recipient, fixed employee roster, Mission, and Task states. The product API rejects missing or null `actionable`; `false` explicitly selects FYI. Actionable messages persist the Obligation, work signal, and schedule wake transactionally. FYIs create no Obligation or wake and advance in event order after acknowledgement.

The exact @7 manifest is `82d7b2dbc41ff3dbed56813b3b3adcfad48818fb29653bcd2debf1f280e507eb`; the aggregate schema is 3503 bytes with digest `769f7c9f4afb1c1d0ebfb43037a06d8f2ddb661bc111970c4d199f48f962fd42`. Only the exact zero-egress Fake Runtime purpose/envelope/markers accept @7. The real-provider path remains pinned to @4. Product-provider Task execution is still restricted to `compat/emp-backend`; this slice does not qualify other fixed roles or any real provider interaction. Schema remains 72; no migration was added.

Verification completed:

- `rtk bash scripts/go.sh test ./internal/codex ./internal/kernel ./internal/provider ./internal/control`
- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` on a dedicated disposable PostgreSQL 18 instance through Schema 72, including send/submission and send/pause event-order checks
- `rtk bash scripts/go.sh test ./...`
- `rtk bash scripts/go.sh build ./cmd/...` for Linux amd64
- `rtk bash -lc 'GOOS=windows GOARCH=amd64 ./scripts/go.sh build ./cmd/...'`
- `rtk git diff --check` after the final handoff edit.

Evidence: `evidence/development/r1-r3-implementation-validation-20260930-slice-101-product-direct-worker/verification.md`.

## Remaining work and next move

Use the existing finite list in `R1_R3_IMPLEMENTATION_COVERAGE.md`. It still includes REQ-13 fairness/quota readiness and automatic Worker admission/dispatch; lifecycle safety for REQ-14/15/16/25/26/29/39; the qualified employee/runtime/continuity path for REQ-23/24/27/30–34; signing and clean Windows VM/package checks; one bounded FT/NT/CAP/UI/WF/PP traceability reconciliation; R2 Linux host/recovery and remote-workbench qualifications; and independent R3 content/research quality, cost, recovery, and organization-benefit evidence.

Recommended next slice: implement the REQ-13 fairness/quota admission and automatic Worker dispatch path for the current fixed product Task. First inspect the schedule reconciliation, `EmployeeSchedule`, and Routine/Task delivery tests; keep the dispatcher bounded and preserve pause/quota barriers. Qualification and real provider work stay separate. Update the coverage ledger and evidence at the end of that slice.

## Working conventions

- Follow `AGENTS.md`: Go/PostgreSQL 18/pgx/sqlc/goose, migrations only forward, evidence under `evidence/development/`, and RTK-prefixed shell commands.
- Preserve exact provider-surface fingerprints and old probe registries. A new model-visible tool set needs a distinct versioned surface and independent qualification.
- Use disposable PostgreSQL/file roots only. Full Go suite and both Linux/Windows command builds are appropriate after code slices; external qualification remains gated.
- There is one principal code writer. Review is read-only. Wait for reviewers instead of terminating them early.
- The reviewed Slice101 commits and this handoff are synchronized to this GitHub `main` for cloud pickup. This is source synchronization only; it does not authorize deployment or external provider actions.
