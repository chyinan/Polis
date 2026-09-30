# Slice 29 verification — R3 domain evidence readiness

Date: 2026-09-25

## Changes covered

- `content-operations@1` requires quality, intervention, recovery, cost and organization-benefit evidence. `research-simulation@1` requires quality, recovery, cost and organization-benefit evidence.
- Each area record binds an artifact ID/revision/SHA-256, assessment-method SHA-256 and assessor employee ID to the exact known workflow profile revision.
- Structural readiness rejects unknown profile revisions, unrecognized or duplicate areas, and malformed artifact/method/assessor references. Missing required areas report `incomplete`.
- Complete records return `ready_for_review` only. The profile remains `not_run`, execution remains disabled, and this contract does not decide evidence quality or invent acceptance thresholds.

## Verification results

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test -race ./internal/domainworkflow -count=1` | PASS. Both profile configurations, complete/incomplete submissions, unknown/duplicate areas, unknown revisions, malformed artifact refs, bad method digests and invalid assessors are covered. |
| `bash scripts/go.sh test ./... -count=1` | PASS. Database integration cases without a dedicated DSN were skipped. |
| `bash scripts/go.sh build ./cmd/...` | PASS for Linux. |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS. |
| `git diff --check` | PASS. |

No real content/research data, external account or provider was used. The evidence evaluator is a pure local contract; no database persistence, Workbench submission API or domain qualification was added. Both domain profiles remain unavailable until domain-specific evidence is independently reviewed and recorded.
