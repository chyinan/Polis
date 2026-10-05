# Polis cloud Codex handoff

Updated: 2026-10-06

## Current continuation pointer (Slice 263)

CSV input now has bounded parsing and a revision/manifest/source-digest-bound table summary. An authenticated backend row-range reader checks the active DB-bound WorkerSession and exact current Task source, caps results by rows and bytes, and audits metadata without cell contents. The tool is isolated to unqualified fake `polis-product-tool-surface@13`; real provider `@4` remains unchanged. PDF raster/image receipts, the full R2 format matrix and runtime/provider qualification, approved fixtures, and authorized active-Worker delivery remain open. Go build, migration hash validation for all 110 migrations, and diff check passed; no tests, DB, Worker, provider, or scenario ran. REQ-35 remains partial; 18 software requirements remain open and all 232 scenarios remain `not_run`. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-263-req35-csv-table-range/verification.md.

Source is Schema 110. The local runtime and last recorded database observation remain at Schema 108; no migration was applied for this stage.

## Previous continuation pointer (Slice 262)

Schema 110 adds an installation-wide Worker slot policy with no default cap/reserve, immutable revision events, and durable slot-class reservations. Both production admission paths serialize on the singleton policy row and insert the reservation with the WorkerSession; active sessions are counted across Companies. The Installation Owner API and page expose an authenticated, CSRF-protected, revision-checked configuration form. Unconfigured policy keeps admission fail-closed; reducing capacity does not stop existing sessions. `review` and `peer_review` use protected slots. Provider quota readiness and `waiting_quota` release remain unimplemented. Go build, migration-hash validation for 110 migrations, frontend production build, and diff check passed; no tests, DB, Worker/provider action, or scenario ran. REQ-13 remains partial; all 232 scenarios remain `not_run`, and 17 open REQs are unchanged. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-262-req13-installation-worker-slots/verification.md.

Source migration is Schema 110. The local runtime and last recorded database observation remain at Schema 108; no database migration was applied for this stage. The new policy is deliberately unset until the installation owner configures it.

## Previous continuation pointer (Slice 261)

The exact reviewed fixed-team matrix now compiles to a typed content-addressed team-matrix revision. Owner-confirmation events retain the exact bytes, and Company summary projections verify the current digest, decision and unverified state before exposing it. The semantic resolver reports assignments but remains human-required while execution is disabled and qualifications are unverified. This is not a per-employee RoleRevision; no semantic task_type-to-TaskKind mapping or admission behavior was added. C-AUTHORITY still needs an approved Task/PlanRevision semantic binding and explicit mapping. Go build, frontend production build and diff check passed; no tests, DB, Worker/provider action or scenario ran. Seven REQ-02 rows remain partial/not_run, all 232 scenarios remain not_run, and 17 open REQs are unchanged. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-261-req02-semantic-role-revision/verification.md.

The local runtime remains at the Slice257 build and database observation (Schema 108); source is Schema 109. Do not apply the new migration as part of this source/evidence stage.

## Previous continuation pointer (Slice 260)

Controlled stdio and Streamable HTTP MCP calls now require an expiring, single-use permit bound to the exact action/attempt, Task and WorkerSession generation, Employee epoch, current binding revision, runtime qualification, target fingerprint, and argument digest. Permit consumption rechecks authority 