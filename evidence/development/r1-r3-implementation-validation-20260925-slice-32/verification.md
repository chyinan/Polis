# Slice 32 verification — independent R3 evidence-reference review

Date: 2026-09-25

## Implemented scope

- Schema 30 adds an append-only, company-scoped review table with one decision per evidence submission. Outcomes are `evidence_references_accepted`, `evidence_references_rejected`, and `more_evidence_required`.
- Kernel validation and the database insert trigger require a `ready_for_review` submission, a same-company employee with `role_name='review'`, and a reviewer identity distinct from every assessor on the submission. The trigger holds a `FOR SHARE` row lock while checking the reviewer role.
- Reviews require a nonblank rationale no longer than 2000 characters. RequestID replay is idempotent; a second decision conflicts. Direct updates and deletes are rejected.
- The company evidence ledger projects review state. A session-token-protected Workbench POST route records decisions, and the Group Settings panel shows the reference metadata and review outcome.
- These decisions only concern reference structure. Evidence bytes are not previewed; the UI states that decisions do not attest to content quality. Qualification remains `not_run` and execution remains disabled.

## Verification results

- `rtk proxy bash scripts/r3-domain-evidence-postgres-test.sh --race` — PASS after the final migration change. Goose migrated a fresh, uniquely named temporary PostgreSQL 18 cluster through schema 30. Kernel and Workbench integration tests passed under the race detector. Coverage includes independent and wrong-role reviewer checks, same-reviewer-as-assessor rejection at both Kernel and DB-trigger layers, incomplete submission rejection, one-decision uniqueness, idempotent replay, append-only enforcement, company isolation, ledger projection, and qualification/execution staying disabled. The script dropped its temporary database, stopped the cluster and removed only its exact temporary root.
- `rtk proxy bash scripts/go.sh test ./... -count=1` — PASS across the complete Go suite. This ran before the last SQL-only `FOR SHARE` lock strengthening; the final post-change PostgreSQL race integration passed afterward.
- `rtk proxy bash scripts/go.sh test ./internal/domainworkflow ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop -run DomainEvidence -count=1` — PASS after the final Go and route changes.
- `rtk proxy bash scripts/go.sh build ./cmd/...` — PASS for Linux commands.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` — PASS for Windows amd64 commands.
- `rtk proxy npm --prefix frontend run test -- --run` — PASS, 74 tests across 12 files.
- `rtk proxy npm --prefix frontend run typecheck` — PASS.
- `rtk proxy npm --prefix frontend run lint` — PASS.
- `rtk proxy npm --prefix frontend run build` — PASS. Vite reports the existing large-chunk advisory (562.83 kB minified JavaScript).
- Read-only code review — initial review found no Critical or Important issues and one Minor concurrency-lock issue. The migration now uses `FOR SHARE`; reviewer follow-up confirmed that it blocks concurrent `role_name` updates and found no remaining or new issues. The reviewer ran no tests/builds and modified no files.

## Scope and qualification limits

No external service, production database, business account, real content/research evidence, model, QQ, MCP endpoint or GitHub account was used. Neither reference profile is qualified. Evidence-content preview and substantive domain qualification remain open.
