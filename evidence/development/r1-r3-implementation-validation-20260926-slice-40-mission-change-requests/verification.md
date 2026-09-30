# Slice 40 Mission change request verification

Date: 2026-09-26

## Implemented behavior

- Schema 36 stores immutable Mission change proposals, append-only state events, revisioned impact snapshots, and output block records. The base digest binds Mission title, goal, acceptance criteria and each latest MissionInput ID/revision/state/content hash.
- Impact snapshots list affected Tasks and workspaces, Artifacts, active WorkerSessions, nonterminal JobRuns, unrevoked service endpoints and exact input revisions. Each collection is capped at 512 records and successor workspace carryover is capped at 64 KiB total. Natural-language impact remains explicitly `not_assessed`.
- Requests on active Missions queue. Consideration requires a paused Mission and no live WorkerSession, unresolved JobRun, service lease, or uploading/stored input. Application requires the exact reviewed impact digest and rechecks the requirements base and impact inside the applying transaction.
- Applying cancels the old Mission and unfinished Tasks, retains their immutable history, creates a draft successor with the approved goal and acceptance contract, clones latest MissionInput references by CAS hash, and carries unfinished Task workspace blobs into the successor as handover inputs. The applied event records the input/workspace source map.
- If requested, manifests and ZIP downloads for old Artifacts are blocked while the request is open or applied; the company overview projects affected Artifacts as invalidated and Task acceptance as inconclusive. Declining reopens delivery and restores prior projections. The Workbench supports request, list, consider, decline and apply actions, displays exact impact counts and stop requirements, and reports that natural-language impact was not assessed.

## Verification

| Command / exercise | Result |
|---|---|
| `rtk proxy bash scripts/r1-capability-source-postgres-test.sh` | PASS; disposable PostgreSQL 18 migrates through Schema 36; 34–36 upgrade/rollback guards pass; Kernel integration covers active-writer denial, stopped-boundary impact review, one-open-request policy, stale-impact rejection, old-result blocks, successor requirement/input/workspace transfer and new Task manifest binding; Workbench verifies blocked manifest/ZIP and delivery after decline |
| `rtk proxy bash scripts/go.sh test -p 1 ./... -count=1` | PASS; full serial Go suite |
| `rtk proxy npm --prefix frontend test` | PASS; 90 tests |
| `rtk proxy npm --prefix frontend run typecheck` | PASS |
| `rtk proxy npm --prefix frontend run lint` | PASS |
| `rtk proxy npm --prefix frontend run build` | PASS; existing Vite advisory remains for the ~596 kB main JS chunk |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 command build |
| Windows amd64 `go test -c` for `internal/kernel`, `internal/workbench`, `internal/provider`, and `internal/control` | PASS; test binaries compiled and removed |
| `rtk proxy git diff --check` | PASS; only existing CRLF conversion advisories |

No real provider/model turn, QQ send, external MCP endpoint, GitHub account, WFP/loopback system change, or production action was used. The real provider surface remains gated. Mission-level changes are implemented; task-specific selective revision, automated natural-language impact analysis, and human workspace write leases remain open. The successor Mission is deliberately left as a draft for the operator to start.
