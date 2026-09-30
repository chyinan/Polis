# Slice 39 operator guidance verification

Date: 2026-09-26

## Changes covered

- Schema 34 adds append-only responses unique per operator-instruction recipient. Responses bind the active Employee, WorkerSession and Task, use one of `applied`, `rejected`, or `needs_clarification`, retain a bounded summary and are idempotent by RequestID. Schema 35 materializes recipients and allows a nullable Mission only for explicit company-wide guidance. A broadcast remains pending until every frozen recipient responds; its aggregate terminal status reflects those responses. Database triggers reject response/recipient update/delete, and both down migrations refuse to discard stored history/scope.
- `guidance_read` rechecks the active WorkerSession and returns pending instructions for the current Mission, Task and Employee plus company-wide items from earlier or future Missions in the same company. A recipient stops seeing an item after responding while other recipients continue to see it. Mission-specific guidance stays within its Mission. The model cannot supply its own Employee, Mission or Task identity.
- Creating an instruction with both Task and Employee targets verifies that the selected, enabled Employee owns that Task. Mission-scoped employee-only guidance requires at least one Task assigned to that Employee in the Mission; task and mission broadcasts snapshot only enabled owners. The Workbench filters incompatible Task/Employee targets before submission.
- The schema 35 upgrade preflight refuses legacy Employee/Task mismatches, mission guidance targeted to an Employee without an assigned Task, and pending guidance without any eligible recipient. The temporary PostgreSQL migration test downgrades/repairs schema 34/35, exercises each rejected legacy case, verifies enabled-only recipient backfill, and confirms both downgrade guards refuse stored recipient/response history.
- `guidance_respond` rechecks the active binding and frozen recipient assignment in the write transaction. Cross-employee and duplicate-per-recipient responses are denied; free text does not change Task scope, validation bindings, approvals or permissions.
- Workbench instruction readback includes response outcome, summary, responder and timestamp.
- Product surface v6 adds the two guidance operations on top of the separate v5 Skill surface. Historical v4 and v5 manifests remain unchanged. The current provider authorization gate still rejects v6.

## Verification

| Command / exercise | Result |
|---|---|
| `rtk proxy bash scripts/r1-capability-source-postgres-test.sh` | PASS; disposable PostgreSQL 18 migrates through Schema 35; populated schema-34 upgrade tests cover 3 unsafe legacy cases, an enabled-only valid recipient snapshot, schema 34/35 downgrade success and guarded refusal with response/recipient history; kernel integration covers cross-Mission broadcast, Mission isolation, disabled/unassigned target rejection, per-recipient replay/uniqueness, the multibyte summary boundary and immutable history; Workbench projects a partial broadcast as pending with recipient responses |
| `rtk proxy bash scripts/go.sh test -p 1 ./... -count=1` | PASS; full serial Go suite |
| `rtk proxy npm --prefix frontend test` | PASS; 87 tests |
| `rtk proxy npm run --prefix frontend typecheck` | PASS |
| `rtk proxy npm run --prefix frontend lint` | PASS |
| `rtk proxy npm run --prefix frontend build` | PASS; existing Vite advisory remains for the ~581 kB main JS chunk |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 command build |
| Windows amd64 `go test -c` for `internal/kernel`, `internal/workbench`, `internal/provider`, `internal/control` | PASS; test binaries compiled; temporary binaries were removed |
| `rtk proxy git diff --check` | PASS; only existing CRLF conversion advisories |

Response summaries use 128 Unicode characters and at most 512 UTF-8 bytes consistently in the v6 schema, Go validator, PostgreSQL constraint and Workbench validator; whitespace-only summaries are rejected at both schema and runtime. Tests cover the multibyte boundary and impossible response-state combinations. The v5 Skill surface manifest and schema digests remain pinned to their prior values. Frontend production build output remains about 581 kB for the main JavaScript chunk and reports the existing Vite advisory. No model turn, QQ send, external MCP endpoint, GitHub account or production action was performed. Guidance response semantics are recorded and locally verified, but v6 has not been qualified against a live provider. Formal requirement-change invalidation, write handback and live employee execution remain open.
