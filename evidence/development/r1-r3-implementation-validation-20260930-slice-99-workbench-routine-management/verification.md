# Slice 99 verification — Workbench Routine management

Date: 2026-09-30

The Workbench now lists Routines scoped to a selected Mission, creates a daily Routine with a required fixed task instruction, and repairs legacy `needs_instruction` rows through an idempotent authenticated command. The PostgreSQL integration exercised create/readback and legacy occurrence→repair→linked Task readback. No schema migration, automatic Worker dispatch, real Worker, model request, or external provider was used.

| Verification | Result |
| --- | --- |
| `bash scripts/r1-employee-schedule-postgres-test.sh` | PASS; disposable PostgreSQL 18, schema 60→72, including Workbench HTTP create/read/legacy repair readback |
| `bash scripts/go.sh test ./...` | PASS; all Go packages |
| `bash scripts/go.sh build ./cmd/...` | PASS; Linux amd64 commands |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 commands |
| `npm --prefix frontend run typecheck` | PASS |
| `npm --prefix frontend run test` | PASS; 16 files, 139 tests |
| `npm --prefix frontend run lint` | PASS; zero warnings |
| `npm --prefix frontend run build` | PASS; retains the existing >500 kB chunk advisory |
| `scripts/test-migration-hash-manifest.ps1` | PASS |
| `git diff --check` | PASS |

Independent read-only code review of the Slice 99 API, scope checks, UI flow, and tests found zero actionable issues.

The Routine task instruction is immutable after creation. The repair route only applies to a Routine belonging to the selected Company and Mission; Tasks created by repair remain subject to Mission/schedule admission barriers. Routine fairness/quota policy and automatic Worker dispatch remain outside this slice.
