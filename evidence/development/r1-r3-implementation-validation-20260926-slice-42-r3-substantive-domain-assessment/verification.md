# Slice 42 — R3 per-submission substantive domain assessment

Date: 2026-09-26

## Implemented

- Added a new incremental Schema 38 ledger for one immutable substantive assessment per ready R3 evidence submission, with normalized per-area rows, preview attestations, reviewer identity, evidence digest and a guarded downgrade.
- Kept reference-integrity review separate. A substantive assessment requires an accepted Schema 31 preview-bound reference review, a reviewer with the company `review` role who is independent of every assessor, and fresh per-area preview attestations.
- The Kernel re-reads each preview from company CAS, verifies source and content hashes, validates the exact content or research evidence-area set, persists accepted/rejected/insufficient judgments with bounded rationale, and derives the overall case result. Schema triggers independently verify reviewer/assessor separation, source/profile coverage, JSON/normalized-row correspondence and overall-result derivation.
- Added company-scoped token-gated Workbench POST `/domain-evidence/{recordId}/assessment`, RealWorkbenchApi validation, query invalidation and per-area reviewer controls in Group Settings.
- Case-level results do not alter profile qualification or execution. Both reference profiles continue to report `not_run` and disabled execution.

## Verification

- `rtk proxy bash scripts/r3-domain-evidence-postgres-test.sh` — passed. A dedicated temporary PostgreSQL cluster migrated through Schema 38; the populated assessment downgrade guard, content-operations and research-simulation Kernel cases, CAS preview binding, Workbench POST routing, and company-scoped evidence ledger passed.
- `rtk proxy bash scripts/r1-capability-source-postgres-test.sh` — passed against Schema 38. This re-ran the existing Schema 35/36/37 migration guards plus Schema 38 and the R1 capability/takeover regression set under race detection.
- `rtk proxy bash scripts/go.sh test ./...` — passed across the full Go module.
- `rtk proxy npm test` — passed, 12 files and 93 tests.
- `rtk proxy npm run typecheck` — passed.
- `rtk proxy npm run lint` — passed.
- `rtk proxy npm run build` — passed. Vite reports the single JS bundle at 619.69 kB minified, above its 500 kB advisory threshold.

No real content-operations or research evidence, external account, model, QQ, MCP, GitHub or production action was used. These are human judgments attached to a specific evidence package, not global domain qualification; the two profiles remain unavailable and execution-disabled pending real domain evidence and separate qualification.
