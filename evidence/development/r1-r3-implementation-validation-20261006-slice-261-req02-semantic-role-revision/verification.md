# Slice 261 verification — REQ-02 semantic team-matrix revision

Date: 2026-10-06

## Scope

The exact reviewed `TEAM_COVERAGE.json` now compiles to a typed, content-addressed team-matrix revision. Owner-confirmation events store the exact source bytes in the same Company transaction; Company summary reads validate the digest, current embedded bytes, owner decision, and `unverified` qualification before returning the typed projection. Existing exact-digest events without a byte snapshot are reconstructed only from the same current embedded bytes. The pure semantic resolver reports the covered task type, owner, checker set, and acceptance path; current draft qualification and disabled execution keep every resolution on the human-required path.

This SHA-256 revision is a team-wide matrix revision, not the missing per-employee ordinal RoleRevision. No semantic `task_type` was mapped to persisted `TaskKind`, and Worker admission behavior from Slice 259 is unchanged. The C-AUTHORITY contract still needs an approved Task/PlanRevision semantic binding and explicit mapping before role resolution can control admission.

## REQ-13 audit

The current source has the shared Company fairness cursor and per-Task claims, but no owner-selected installation-wide cap or provider-authoritative quota readiness/recovery source. No cap or quota release semantics were guessed; `waiting_quota` remains fail-closed.

## Verification

- `go build ./...`: passed.
- `npm run build` in `frontend/`: passed (`tsc -b` and Vite production build); the existing >500 kB bundle advisory remains.
- `git diff --check`: passed.
- No tests, database reads/writes, Worker activity, provider calls, or frozen scenarios ran.
- All seven REQ-02 rows remain `partial/not_run`; all 232 scenario executions remain `not_run`. The 17 open software requirement IDs are unchanged.
