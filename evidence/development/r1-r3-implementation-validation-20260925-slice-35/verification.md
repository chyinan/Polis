# Slice 35 — bound read-only Skill loading foundation

Date: 2026-09-25

## Implemented

- Added a separate eight-tool `polis-product-tool-surface@5` definition with `polis_skills_load`. The existing qualified v4 surface and its manifest and `work_current` output remain unchanged; only v5 exposes the bound Skill catalog and load history.
- Only v5 `work_current` includes approved, metadata-verified Skill revisions bound to the active employee, with exact revision digests and package file references. The v4 `work_current` result omits the new catalog/history fields, preserving its prior response shape. Neither surface includes Skill bodies until `skills_load` is called.
- `skills_load` is accepted only by the v5 EmployeeTools dispatch. The Kernel requires an active WorkerSession, current same-company Employee binding, exact version digest, `metadata_verified` qualification, current `approved` decision and the fixed read-only ZIP source.
- The selected Markdown/plain-text file is returned only after all package CAS bytes are read and the canonical package manifest, sizes, paths and hashes are verified. A load is limited to 64 KiB of text. PNG/JPEG assets are listed as non-loadable by this text operation.
- Each first load appends a company event with Employee, Task, session, Skill revision, path, media type, and content digests, without copying the Skill body. Repeated loads of the same path in one session reuse the same load reference. Handover carries the bound Skill catalog and recent Task load references. Revocation/unbind is rechecked for each request, Kernel receipt replay, and native transport replay; the replay recheck holds the same company write lock as revocation through the CAS read.
- Loaded text is explicitly marked as static content with no added permissions. The v5 thread instructions state it cannot override Polis authorization or the Task contract. No script or dependency execution was added.
- There is no database migration; schema remains 31. The current provider authorization remains v4-only, so real provider turns with v5 are rejected and were not attempted.

## Verification

| Command | Result |
|---|---|
| `bash scripts/r1-capability-source-postgres-test.sh` | PASS; fresh temporary PostgreSQL 18 cluster migrated through Schema 31; approved/bound Skill load, unbound denial, exact text/digest return, same-session load-reference reuse, handover projection, employee unbind, and denial after revoke including identical-call replay pass under race detection. Existing source-CAS, approval, and same/distinct-revision concurrency cases also pass. The script removed only its uniquely named temporary cluster/database. |
| `bash scripts/go.sh test ./internal/codex -run TestProductEmployeeSkillSurfaceAddsOnlyBoundedReadOnlyLoad -count=1` | PASS; v4 definitions stay unchanged and v5 adds only the closed-schema Skill text loader. |
| `bash scripts/go.sh test ./internal/provider -run TestReadOnlySkillSurfaceIsSeparateAndRemainsUnqualified -count=1` | PASS; v5 has a distinct manifest and the current authorization gate rejects it. |
| `bash scripts/go.sh test ./internal/provider -run TestProductSkillInstructionsConstrainLoadedStaticText -count=1` | PASS; v5 instructions constrain Skills to static guidance. |
| `bash scripts/go.sh test ./internal/codex -run TestSkillLoadReplayRechecksCurrentAuthorization -count=1` | PASS; the native transport redispatches a duplicate Skill load to the authorization handler instead of replaying cached content. Existing duplicate workspace reads remain cached and delivered once. |
| `bash scripts/go.sh test -p 1 ./... -count=1` | PASS; full serial Go suite. |
| `bash scripts/go.sh build ./cmd/...` | PASS; Linux command packages. |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 cross-build. |
| `.tools/go/bin/gofmt -d` on changed Go files | PASS; no formatting diff. |
| `git diff --check` | PASS. |

## Review

A focused read-only code review found no remaining Critical, Important or Minor issues. It verified the company write lock spans capability reauthorization and CAS read on Kernel receipt replay; the native transport redispatches repeated `skills_load` calls; the v4 `work_current` JSON result remains unchanged; and only the exact v5 surface enables Skill catalog/load output. No code was changed by the reviewer.

Frontend files were not changed in this slice. No model, QQ, MCP, GitHub, registry, production account or project code was run. Skill surface v5 remains unqualified; the text-only limitation for raster assets remains explicit.
