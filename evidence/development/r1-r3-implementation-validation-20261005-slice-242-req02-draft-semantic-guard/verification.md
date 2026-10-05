# Slice 242 — REQ-02 fixed team draft semantic guard

## Change

Added `spec.ValidateFixedTeamCoverageDraft`, a strict semantic check for the embedded fixed team coverage draft. It checks the canonical roster and seven assignments, owner/checker/acceptance-path mapping, draft kind/version, `execution_enabled=false`, `role_changes_at_runtime=false`, human-confirmation gates, unverified qualifications, and trusted-baseline/independence flags. Unknown fields, missing fields, trailing JSON, or changed semantics fail closed.

Company creation and owner acknowledgment reject an invalid embedded draft; Company detail and switcher projections do not report it as confirmed. Task admission and provider execution policy are unchanged. An owner acknowledgment remains only an acknowledgment of the reviewed draft and never qualifies a role or enables execution.

## Verification

- `go build ./...` passed.
- `git diff --check HEAD^ HEAD` passed.
- Parsed `TEAM_COVERAGE.json`: fixed draft kind, execution disabled, runtime role changes disabled, human confirmation required, seven assignments, all qualifications `unverified`.
- Traceability remains 17 open requirements; all 232 scenarios remain `not_run`.
- No tests, database operations, Company creation/acknowledgment, Worker/provider operation, or frozen scenario ran.
