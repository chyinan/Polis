# Slice 259 — REQ-02 fixed-team Worker admission gate

Date: 2026-10-06  
Source commit: `129d5919f9888724ad6038f47a6e203ca90e1c6d`

## Change

The authoritative WorkerSession admission transaction now requires a current owner confirmation for the exact embedded fixed-team coverage matrix. It checks the latest confirmation event's matrix digest, exact decision (`installation_owner_confirmed_fixed_team_mapping`), and `qualification=unverified`, then verifies that enabled persisted employees still match the canonical fixed roster. A missing, stale, malformed, or mismatched confirmation denies admission before budget reservation and WorkerSession insertion.

The check uses the existing company transaction guard, so confirmation, roster changes, and WorkerSession admission serialize on the Company row. It does not assign semantic task types to persisted `TaskKind` values and does not qualify employee roles or provider execution.

## Verification

- `go build ./...` — passed.
- `git diff HEAD^ HEAD --check` — passed.
- No tests were run.
- No database was accessed, no WorkerSession was created or used, and no provider or frozen scenario was exercised.
- All seven REQ-02 scenario rows remain `partial/not_run`; all 232 scenario executions remain `not_run`. Seventeen software requirements remain open.

The local development binary was last refreshed at Slice 257 and has not yet been rebuilt from this source commit.
