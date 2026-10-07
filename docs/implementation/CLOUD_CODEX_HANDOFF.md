# Polis cloud Codex handoff

Updated: 2026-10-07

## Current continuation pointer (Slice 305)

The authorized browser qualification was rerun with a Windows-hosted,
cross-compiled service fixture so browser and service shared one loopback
namespace. Windows Chrome rendered the service page and completed the
same-origin fetch successfully. Effective WFP/profile isolation, packaged
Node/npm, clean-VM and production Provider qualification remain open. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-305-windows-browser-fixture-qualification/verification.md`.

## Current continuation pointer (Slice 304)

Authorized local browser requalification reran the WSL service fixture with
installed Windows Chrome. The fixed Workbench fixture was reachable, but the
randomized WSL `127/8` service ingress was refused from Windows, reproducing
Slice280. The fixture was cleaned up; no browser isolation or rendering pass
was claimed and no source code changed. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-304-local-browser-requalification/verification.md`.

## Current continuation pointer (Slice 303)

Extended the canonical evidence binding to succeeded ResearchOperation
results. The envelope hash, Company/Mission/Task/Operation scope and real
ready candidate/passed Artifact digests are checked fail-closed; unavailable
fake search/fetch remains unchanged. No retrieval, browser, network or
Provider path was enabled. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-303-req38-research-evidence-binding/verification.md`.

## Current continuation pointer (Slice 302)

Added fail-closed BrowserRun evidence binding. Succeeded records now require a
canonicalized, content-addressed, same Company/Mission/Task/Run evidence
manifest whose real Artifact rows match scope/digest and ready candidate/passed
state; blocked records remain explicit. No browser, network, credential,
Provider or external-account path was enabled. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-302-req38-operation-evidence-binding/verification.md`.

## Current continuation pointer (Slice 301)

Added the optional read-only TaskSummary semantic TaskRevision projection.
Company/Mission/Task scope, Task owner/kind alignment, digest/qualification
validation and latest decision state are checked in the read store; older
schemas remain compatible. Frontend validation and display are read-only, with
no approval or execution control. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-301-req02-task-revision-projection/verification.md`.

## Current continuation pointer (Slice 300)

Added the monotonic TaskRevision owner-decision lifecycle and Schema123
append-only decision ledger. Only `proposed -> approved|rejected` and
`approved -> revoked` are accepted; rejected/revoked rows cannot be
resurrected, and `qualified` is not a decision state. The Kernel exact-binding
command is exposed through an installation-owner/CSRF Workbench route, while
qualification and execution remain unchanged and denied. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-300-req02-task-revision-decisions/verification.md`.

## Current continuation pointer (Slice 299)

Added Schema122 append-only `task_semantic_revisions` context. The product
Worker admission transaction records the exact `compat`/Backend Task binding
to the owner-confirmed RoleRevision when the optional table exists, with
idempotent insert/replay and same-transaction read-back. Rows remain
`unverified`/`requires_human=true`; this does not authorize execution. Absence
of Schema122 skips only this optional write; the existing Schema121 admission
gate remains required. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-299-req02-task-semantic-revisions/verification.md`.

## Current continuation pointer (Slice 298)

Added the pure, content-addressed REQ-02 semantic TaskRevision binding from a
fixed-team RoleRevision to an explicit `task_type`/owner/TaskKind tuple.
Mismatches return distinct reason codes; exact matches remain
`qualification=unverified` and `requires_human=true`. No durable revision,
Worker admission, Provider execution or external qualification was enabled.
Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-298-req02-task-revision-binding/verification.md`.

## Current continuation pointer (Slice 297)

Aligned stale offline test fixtures with the current CreateMission, one-shot
MCP dispatch-permit, Streamable HTTP permit-callback, and Codex identity
readiness contracts. Production behavior and qualification gates are
unchanged. The full Go package suite and command build pass; no database
runtime, Worker/provider, browser, external account, MCP endpoint or frozen
scenario ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-297-offline-test-contracts/verification.md`.

The full offline Go test suite is clean; process-level MCP tests use an
explicit fake permit callback and do not qualify a real endpoint. Remaining work is gated by executable
Role/TaskRevision/provider qualification, action-specific generic action
policy, native executor/service recovery, isolated BrowserRun/research
execution and evidence binding, migration/runtime qualification, and external
account/provider/scenario evidence.

## Current continuation pointer (Slice 296)

Slice296 adds optional Schema120 generic ActionIntent denied-audit projection to Workbench Operations. Latest denied events are Company-scoped and bounded before limiting; only ResourceKey/digest metadata is exposed and older schemas omit the field. No external action, Provider, browser, database runtime, account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-296-action-intent-operations-projection/verification.md`.

## Current continuation pointer (Slice 295)

Slice295 extends optional Schema121 EmployeeSummary RoleRevision readback with role name, semantic task types/kinds, owner decision and qualification metadata. Old schemas return null; malformed detail fails closed and no execution qualification changes. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-295-role-revision-detail-projection/verification.md`.

## Current continuation pointer (Slice 294)

Slice294 tightens frontend EmployeeSummary RoleRevision validation to null or lowercase SHA-256 and corrects a stale MissionChangeRequest fixture. Full validation tests (50/50), TypeScript and Vite builds pass; no backend/runtime/provider/browser/account/scenario action ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-294-role-revision-frontend-boundary/verification.md`.

## Current continuation pointer (Slice 293)

Slice293 adds optional Schema121 `EmployeeSummary.roleRevision` projection, selecting the latest Company/Employee revision and returning null on older schemas. It is read-only and does not affect admission or execution. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-293-req02-role-revision-projection/verification.md`.

## Current continuation pointer (Slice 292)

Slice292 adds Schema121 immutable per-employee RoleRevision rows written with fixed-team owner confirmation. Rows bind the reviewed matrix digest, role name, semantic task types and descriptive TaskKinds while remaining `unverified`; Worker admission and provider execution are unchanged. No database runtime, owner action, Worker/provider, browser, account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-292-req02-role-revisions/verification.md`.

## Current continuation pointer (Slice 291)

Slice291 adds an explicit descriptive bridge from the fixed team coverage `task_type` vocabulary to persisted TaskKind values. Unknown types remain unmapped and every resolved admission remains `requires_human=true`; no Worker admission or execution capability changed. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-291-req02-semantic-task-kind-bridge/verification.md`.

## Current continuation pointer (Slice 290)

Slice290 adds Schema120 generic ActionIntent default-deny persistence. Strict ResourceKey/target/input/idempotency validation and exact Task/WorkerSession scope are enforced; requests persist `denied/generic_dispatch_permit_unavailable` and never receive a generic DispatchPermit. No external action, Provider, browser, database runtime or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-290-req14-generic-action-intent-deny-foundation/verification.md`.

## Current continuation pointer (Slice 289)

Slice289 adds the optional Schema119 `revisionRoutes` read projection to durable delivery lifecycle responses. It joins immutable Manifest/disposition rows, validates `changes_requested`, bounds results to 16 and renders pending/successor/task-ready states read-only; pre-Schema119 responses remain compatible. No database runtime, Worker/provider, browser, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-289-req40-revision-route-projection/verification.md`.

## Current continuation pointer (Slice 288)

Slice288 adds Schema119 append-only delivery revision routes. Live-Mission `changes_requested` now links the exact Manifest/disposition to the formal MissionChangeRequest, then records `successor_mission_created` and `revision_task_ready` at the existing Apply/Prepare boundaries. Current-Mission parallel Tasks are not created; terminal feedback remains Company backlog. No runtime database, Worker/provider, browser, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-288-req40-delivery-revision-routes/verification.md`.

## Current continuation pointer (Slice 287)

Slice287 extends the REQ-40 durable delivery read projection with bounded Manifest revision and UserDisposition histories. Backend and frontend validation rebind history to Company/Artifact/Mission/Task, stored digests, Artifact bytes and known revision identities; the Workbench renders the history read-only. No delivery command, database runtime, Worker/provider, browser, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-287-req40-delivery-history-projection/verification.md`.

## Current continuation pointer (Slice 286)

Slice286 adds Schema118 ResearchOperation control-plane persistence and fake-only product surface `polis-product-tool-surface@19` with `research_search` and `research_fetch`. Search/HTTPS fetch inputs are bounded and stored as explicit `unavailable/research_backend_unavailable` results with `provider_egress=0`; no query or URL is sent to any backend. No external web, Provider, browser, account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-286-req38-research-unavailable-control-plane/verification.md`.

## Current continuation pointer (Slice 285)

Slice285 adds Schema117 BrowserRun control-plane persistence and fake-only product surface `polis-product-tool-surface@18` with `browser_run` and `browser_results`. Requests are exact Company/Mission/Task/WorkerSession and current service-generation scoped, then persist as `blocked/browser_runtime_unqualified`; HTTPS origin metadata is canonicalized and credentials/query/path fragments are rejected. No URL navigation, browser process, credentials, Provider, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-285-req38-browser-run-control-plane/verification.md`.

## Current continuation pointer (Slice 284)

Slice284 wires the Schema116 BorrowerLease lifecycle into fake-only product surface `polis-product-tool-surface@17` with `jobs_borrow`, `jobs_touch` and `jobs_release`. The control path revalidates active owner/borrower session scope and the exact ready, unexpired service generation; recovery, session stop and generation changes append revocation events. This remains control-plane only: no service process, provider, browser, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-284-req37-borrower-lease-tools/verification.md`.

## Current continuation pointer (Slice 283)

Slice283 adds Schema116 durable BorrowerLease identities/events with exact same-Mission Task/WorkerSession scope, 15-minute TTL, 2-minute idle grace and endpoint/generation revoke paths. It is control-plane lifecycle only; no service process, arbitrary command, provider, browser or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-283-req37-borrower-leases/verification.md`.

## Current continuation pointer (Slice 282)

Slice282 adds Schema115 source and append-only DependencyChange proposal/decision methods. The fixed envelope validates exact semver dependency versions and existing registry hosts; owner approval records intent only and cannot run npm or generate a new environment revision. No migration/runtime database, npm, Worker, executor, provider, browser or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-282-req36-dependency-change-proposal/verification.md`.

## Current continuation pointer (Slice 281)

Slice281 adds owner/CSRF `delivery/invalidate` for exact-current DeliveryManifest revisions, with `invalidated`/`withdrawn` append-only states, owner reason persistence and no Worker/Mission/Task side effect. Artifact content remains immutable; stale/terminal revisions fail closed. Targeted Go/TypeScript tests and frontend build pass; no database runtime, Worker/provider, browser, external account or frozen scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-281-req40-delivery-invalidation/verification.md`.

## Current continuation pointer (Slice 280)

Authorized local executor/browser qualification ran the Windows runner test binary and the service-browser ingress suite. Native runner boundary tests passed; one `POLIS_GO_ROOT` sandbox test skipped. Windows Playwright could not reach the WSL randomized loopback ingress, and WSL Chromium installation could not complete, so browser rendering/effective host isolation remain unqualified. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-280-executor-browser-qualification/verification.md`.

## Current continuation pointer (Slice 279)

Slice279 adds fake-only product surface `polis-product-tool-surface@16` with the existing `environment_status` plus bound `environment_ensure`. Exact revision IDs are checked against the current Mission, and WorkerSession/Task/Mission authorization is rechecked inside the same idempotent write transaction and on receipt replay. No host command, package edit, registry/network selection, real provider, migration, browser or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-279-req36-worker-environment-ensure/verification.md`.

## Current continuation pointer (Slice 278)

Slice278 selects and enforces REQ-40 owner policy: ready delivery completion opens a seven-day `awaiting_feedback` window; expiry rejects later disposition writes without implicit acceptance; successful Mission closeout requires explicit latest `accepted` dispositions for all acceptance Artifacts, while `ended_not_met`/`cancelled` do not. Historical backfill stays `not_requested`. Pre-Schema111 runtimes fail closed for successful closeout. No migration, database runtime, Worker/provider, browser, external account or frozen scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-278-req40-owner-policy/verification.md`.

Owner decisions for REQ-36/37/38 are recorded in `summary/owner-policy-decisions.md`: fixed dependency envelope, same-Mission exact BorrowerLease with 15-minute TTL/2-minute idle grace, and default-deny BrowserRun. These are policy choices, not host/provider/browser qualification.

## Current continuation pointer (Slice 277)

Slice277 adds fake-only product tool surface `polis-product-tool-surface@15` with read-only `jobs_status` and `jobs_logs`. Kernel reads revalidate the current WorkerSession and exact Company/Task/Session ownership in a repeatable-read transaction; log bytes are digest-verified. The qualified real-provider `@4` surface is unchanged. No `jobs.start/stop`, process control, BorrowerLease, real provider, database runtime, browser, external account or frozen scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-277-req37-read-only-jobs/verification.md`.

## Current continuation pointer (Slice 276)

Slice276 connects the owner/CSRF delivery completion command to the real Workbench API and assembling-delivery form. The five evidence sections are bounded and the ready/system receipt is validated; completion remains separate from acceptance and execution. Legacy delivery responses without `feedbackBacklog` remain compatible. Targeted frontend tests/builds pass; no database runtime, Worker/provider, browser, external account or frozen scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-276-req40-delivery-completion-ui/verification.md`.

## Current continuation pointer (Slice 275)

Slice275 adds the explicit `delivery-backfill` management path for legacy deliverable/ready Artifacts without durable Manifest rows. It appends only an `assembling` revision 1 plus `not_requested` disposition and preserves all unavailable evidence sections; it never infers ready, accepted, qualified or Mission completion. The command requires explicit management DSN/blob-root invocation and was not run. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-275-req40-historical-backfill/verification.md`.

## Current continuation pointer (Slice 274)

Slice274 adds the missing read side of terminal delivery feedback: `DurableDeliveryLifecycleResponse.feedbackBacklog` is loaded from Schema113 with Company/Artifact scope, a 32-event bound and fail-closed validation, then displayed in the existing Workbench delivery panel. The read path returns an empty backlog when the optional table is absent on a pre-Schema114 runtime; Schema114 adds a delivery-leading index. It has no Task, Worker, Mission or notification side effect. Targeted frontend validation, frontend build, Go compile/build, DB/hash checks and diff-check passed; no database runtime, Worker/provider, browser, external account or frozen scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-274-req40-backlog-read-projection/verification.md`.

## Current continuation pointer (Slice 273)

Slice273 adds the verified path that Slice272 required: an installation-owner authenticated, CSRF-protected completion command appends a complete `ready` DeliveryManifest revision from immutable source-input, validation/build, run-instruction, limitation and license/source evidence. It refuses stale/forged current manifests, cross-scope Artifact rows, unpassed Artifacts, missing qualification, mismatched task-input/validation-contract digests and malformed evidence. The new revision starts with `not_requested`; no acceptance is inferred from completion, preview, download or notification.

`changes_requested` now routes atomically by Mission lifecycle. Active/paused Missions append the existing formal change-request history; succeeded/ended_not_met/cancelled Missions append the exact delivery/disposition to a new immutable Schema113 Company backlog. Accepted dispositions have no change-routing side effect. Draft/closing/unknown states fail closed and terminal Missions are never resurrected. Feedback deadline/expiry, revision-task routing, closeout acceptance policy and historical backfill remain open. Targeted tests, package compile checks, DB migration/hash checks and diff-check passed; no PostgreSQL runtime, Worker/provider, browser, external account or frozen scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-273-req40-ready-manifest-routing/verification.md`.

## Current continuation pointer (Slice 272)

Slice272 adds an installation-owner authenticated, CSRF-protected append command for explicit `accepted` / `changes_requested` UserDisposition, bound by optimistic manifest and disposition revisions. Kernel uses company-scoped TXWrite idempotency, verifies canonical Manifest SHA, exact Company/Mission/Task/Artifact scope, Artifact `ready/passed`, persisted validation qualification, and formal change-request blockers, then appends the next disposition event with server-selected actor. Workbench only offers the form for `ready`, validates the command receipt, and refreshes that delivery query after success. Current Artifact publication still yields `assembling`, so these commands have no ready target yet. `awaiting_feedback` stays disabled pending deadline/expiry policy; no revision-task/backlog routing, Mission closeout, notification, or Worker side effect was added. `go build ./...`, `npm run build`, and `git diff --check` pass. No tests, migration, DB, Worker/provider/browser, or scenarios ran. Source Schema111; recorded runtime Schema108; all 232 scenarios remain `not_run`. REQ-40 remains partial. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-272-req40-user-disposition-command/verification.md`.

## Previous continuation pointer (Slice 271)

Schema111 adds append-only durable DeliveryManifest revision and independent UserDisposition records, bound to Company/Mission/Task/Artifact/revision. New product Artifact publication writes revision 1 `assembling` and disposition `not_requested` in the Artifact qualification transaction. Workbench exposes a strict read-only lifecycle view; the existing authenticated Artifact ZIP contract remains separate. `go build ./...`, frontend production build, migration hash checks, and `git diff --check` pass. No tests, migration, database, Worker/provider/browser operation, or frozen scenario ran. REQ-40 remains partial; existing pre-Schema111 Artifacts are not backfilled, required source/build/instructions/license sections remain unavailable, and there are no explicit feedback/acceptance/change commands or owner policy. Runtime remains Schema108, source migrations are Schema111, and all 232 frozen scenarios remain `not_run`. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-271-req40-durable-delivery-foundation/verification.md`.

## Previous continuation pointer (Slice 270)

Added the read-only fake-only @14 Worker `environment_status` tool. It uses a repeatable-read snapshot and returns at most 50 environment revision summaries for the bound Task's Mission; the qualified real-provider @4 surface remains unchanged. No tests, migration, database reads/writes, Worker/provider turns, or scenarios ran. Runtime remained Schema108; source migration at that point was Schema110. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-270-req36-worker-environment-status/verification.md`.

## Historical continuation pointer (Slice 268)

MCP and JSON-RPC typed decoding now rejects recursive duplicate keys and case-variant aliases at struct boundaries across stdio, Streamable HTTP JSON/SSE responses and result parsing, controlled mcp_call envelopes, and the reachable fake listed-tool check. Arbitrary map/schema/data keys preserve case-sensitive semantics. Responses require exactly one result or non-null error; result:null remains valid. No dispatch surface or authorization policy changed. Build and diff-check passed; tests, DB, Worker, provider, endpoint, migration, and scenarios did not run. REQ-31/34 stay partial. The final read-only source audit covered all 18 open software requirements and found no safe owner- and environment-independent code slice remaining; all 232 scenarios remain not_run and open rows remain partial. The listed owner/host/provider/active-session/fixture/scenario gates remain. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-268-req31-34-canonical-mcp-json-fields/verification.md and evidence/development/r1-r3-implementation-validation-20261006-slice-268-final-local-audit/verification.md.

## Previous continuation pointer (Slice 267)

The PDF image path now rejects stencil masks, unsupported or partially padded codecs, and unpacked samples whose exact per-row byte length does not match image dimensions. Extraction @3 records image omissions and intrinsic-object limitations; @1/@2 snapshots remain canonical for historical verification, while new active delivery omits the older image representation. Full-page rasterization and provider qualification remain open. Build and diff-check passed; tests, DB, Worker, provider, migration, and scenarios did not run. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-267-req35-pdf-image-fidelity/verification.md.

## Previous continuation pointer (Slice 266)

Streamable HTTP MCP responses are now recursively checked for duplicate JSON object keys before JSON-RPC decoding. The same validator serves direct JSON and SSE responses, covering the envelope and nested tools/schemas/results; the existing shared argument validator was already in place. The change adds no protocol surface or endpoint qualification. `go build ./...` and `git diff --check` passed. No tests, DB, Worker, provider, MCP endpoint, or scenario ran. REQ-31/34 remain partial; all 232 scenario executions remain `not_run`, and 18 software requirements remain open. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-266-req31-34-http-strict-json/verification.md.

Source remains Schema 110 and runtime remains Schema 108; no migration was applied.

## Previous continuation pointer (Slice 265)

The pinned pure-Go PDF parser now yields supported embedded image objects as bounded PNGs in canonical `pdf_snapshot` packages with `polis-pdf-extraction@2` metadata. Each image is tied to page/image number, dimensions, digest, page boxes, rotation, and page-space placement; Task context and immutable delivery receipts carry typed image fields checked against the frozen snapshot. Limits are eight decode attempts, 1M pixels and 256 KiB per image, and 960 KiB total. Legacy extraction-record `@1` snapshots retain their canonical validation path. This does not rasterize/composite complete pages or include vector text/backgrounds, and unsupported image codecs remain omitted. Real-provider image support and the full R2 format/runtime/provider qualification remain open. `go build ./...` and `git diff --check` passed. No tests, DB, Worker, provider, migration, or scenario ran. REQ-35 stays partial; all 232 scenario executions remain `not_run`. Evidence: `docs/implementation/R2_PDF_INPUT_FOUNDATION.md` and `evidence/development/r1-r3-implementation-validation-20261006-slice-265-req35-pdf-embedded-images/verification.md`.

Source remains Schema 110 and runtime remains Schema 108; no migration was applied.

## Previous continuation pointer (Slice 264)

The REQ-29 source audit confirms that `workspace_root` is metadata only, Group membership/shared-root authorization are absent, and snapshot revocation blocks new direct reads without releasing immutable CAS references. Generic orphan collection does not enforce retention, and revoked/corrupt snapshots still count toward the 32-row Task cap. No safe independent implementation can be chosen before owner decisions on root authorization, sharing, writer revocation, and retention plus a qualified native host. The recommended first vertical slice is a read-only Company-root binding on one qualified host with OS-authorized handles, bounded traversal, and digest-pinned snapshots. No code/schema or scenario changed. No tests, DB, Worker, provider, host mount, or frozen scenario ran. See `docs/implementation/REQ29_SHARED_FILE_ACCESS_AUDIT.md` and `evidence/development/r1-r3-implementation-validation-20261006-req29-local-audit/verification.md`.

Source remains Schema 110 and runtime remains Schema 108; no migration has been applied.

## Previous continuation pointer (Slice 263)

CSV input now has bounded parsing and a revision/manifest/source-digest-bound table summary. An authenticated backend row-range reader checks the active DB-bound WorkerSession and exact current Task source, caps results by rows and bytes, and audits metadata without cell contents. The tool is isolated to unqualified fake `polis-product-tool-surface@13`; real provider `@4` remains unchanged. PDF raster/image receipts, the full R2 format matrix and runtime/provider qualification, approved fixtures, and authorized active-Worker delivery remain open. Go build, migration hash validation for all 110 migrations, and diff check passed; no tests, DB, Worker, provider, or scenario ran. REQ-35 remains partial; 18 software requirements remain open and all 232 scenarios remain `not_run`. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-263-req35-csv-table-range/verification.md.

Source is Schema 110. The local runtime and last recorded database observation remain at Schema 108; no migration was applied for this stage.

## Previous continuation pointer (Slice 262)

Schema 110 adds an installation-wide Worker slot policy with no default cap/reserve, immutable revision events, and durable slot-class reservations. Both production admission paths serialize on the singleton policy row and insert the reservation with the WorkerSession; active sessions are counted across Companies. The Installation Owner API and page expose an authenticated, CSRF-protected, revision-checked configuration form. Unconfigured policy keeps admission fail-closed; reducing capacity does not stop existing sessions. `review` and `peer_review` use protected slots. Provider quota readiness and `waiting_quota` release remain unimplemented. Go build, migration-hash validation for 110 migrations, frontend production build, and diff check passed; no tests, DB, Worker/provider action, or scenario ran. REQ-13 remains partial; all 232 scenarios remain `not_run`, and 17 open REQs are unchanged. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-262-req13-installation-worker-slots/verification.md.

Source migration is Schema 110. The local runtime and last recorded database observation remain at Schema 108; no database migration was applied for this stage. The new policy is deliberately unset until the installation owner configures it.

## Previous continuation pointer (Slice 261)

The exact reviewed fixed-team matrix now compiles to a typed content-addressed team-matrix revision. Owner-confirmation events retain the exact bytes, and Company summary projections verify the current digest, decision and unverified state before exposing it. The semantic resolver reports assignments but remains human-required while execution is disabled and qualifications are unverified. This is not a per-employee RoleRevision; no semantic task_type-to-TaskKind mapping or admission behavior was added. C-AUTHORITY still needs an approved Task/PlanRevision semantic binding and explicit mapping. Go build, frontend production build and diff check passed; no tests, DB, Worker/provider action or scenario ran. Seven REQ-02 rows remain partial/not_run, all 232 scenarios remain not_run, and 17 open REQs are unchanged. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-261-req02-semantic-role-revision/verification.md.

The local runtime remains at the Slice257 build and database observation (Schema 108); source is Schema 109. Do not apply the new migration as part of this source/evidence stage.

## Previous continuation pointer (Slice 260)

Controlled stdio and Streamable HTTP MCP calls now require an expiring, single-use permit bound to the exact action/attempt, Task and WorkerSession generation, Employee epoch, current binding revision, runtime qualification, target fingerprint, and argument digest. Permit consumption rechecks authority and atomically writes the consumed permit plus immutable `dispatching` intent under the same Company lifecycle guard as revocation. Unused permits expire; consumed calls interrupted by stop remain `outcome_unknown`. This is limited to controlled MCP: general ActionIntent/DispatchPermit and REQ-26 shared-write bindings remain absent. Schema 109 source is committed, but the local runtime remains on Schema 108; no migration, test, Worker/provider call, or scenario ran. REQ-14 remains partial; all 232 scenarios remain `not_run`. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-260-req14-controlled-mcp-dispatch-permit/verification.md.

The checked-in source is Schema 109, while the local Termux runtime was last built at Slice 257 from Schema 108. Rebuild it from the settled source before relying on the executable; do not apply Schema 109 to the runtime database without the required handoff and migration gate.

Rebuilt `.runtime/bin/polis` from current `main` (Android arm64; embedded VCS revision matches `8ad6f84`) and gracefully restarted the local backend after a read-only check found no Companies, Missions, WorkerSessions, or JobRun events. PostgreSQL reports Schema 108; backend `/healthz` and Vite `/` both return HTTP 200. No Worker, test, provider operation, or frozen scenario ran; all 232 scenarios remain `not_run` and 17 software requirements remain open. Evidence: evidence/development/r1-r3-implementation-validation-20261006-slice-257-local-dev-refresh/verification.md.

The stdio MCP decoder now recursively rejects duplicate JSON object keys in inbound responses, including nested tool definitions and schemas; outgoing tool arguments also reject duplicate keys before validation. This removes last-key-wins ambiguity without changing Streamable HTTP parsing. `go build ./...` and `git diff --check` passed; no tests, database, Worker/provider/MCP endpoint, browser qualification, or frozen scenario ran. The 18 overlapping REQ-31/34 scenario rows remain `partial/not_run`; all 232 scenarios remain `not_run`. Read-only source audits found no other concrete local fail-open in REQ-02/13, REQ-14/15/16, or REQ-27/30/32/33. Host/account/owner and active-session qualification gates remain open. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-256-stdio-duplicate-json-and-audit/verification.md. REQ-23 closeout audit: evidence/development/r1-r3-implementation-validation-20261005-slice-256-req23-closeout-recovery-audit/verification.md.

The InstallationOwner page now routes protected provider-account reads and logout responses through the shared 401/403 observer. Authorization rejection clears the local protected view and broadcasts invalidation to same-origin tabs, including when logout is rejected; login, bootstrap, and setup-status checks remain outside this path. `npm run build` in `frontend/` and `git diff --check` passed. No tests, owner session, database, Worker/provider operation, browser qualification, or frozen scenario ran. Six REQ-25 scenario rows remain partial/not_run and all 232 scenarios remain not_run. Browser/Tauri WebView and remote authorization qualification remain open. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-255-req25-owner-auth-invalidation/verification.md.

The legacy workspace replacement path now validates an existing private tree root under lock, checks its Task owner/Mission and current writer session/epoch, enforces tree bounds, and atomically mirrors `formatter.go` with the legacy workspace row. Submission fails closed unless the tree has one formatter file whose digest and source revision match that row. The six CAP-01–06 rows remain partial/not_run; all 232 frozen scenarios remain not_run. `go build ./...` and `git diff --check` passed. No tests, database, Worker/provider operation, host qualification, or scenario ran. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-254-req29-legacy-tree-writer/verification.md.

## Previous continuation pointer (Slice 253)

Stdio MCP package manifests now reject unknown/case-variant top-level keys before decoding; literal duplicate keys are still rejected and `args` stays optional. `go build ./...` and `git diff --check` pass. No tests, DB, package import, Worker/provider operation, or scenario ran. The 18 mapped REQ-31/34 rows remain partial/not_run; all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-253-req31-34-mcp-manifest-fields/verification.md.

## Previous continuation pointer (Slice 252)

The receipt idempotency path after advisory-lock contention now revalidates the runtime lease and session/Employee epoch in a fresh transaction before returning a matching receipt. `go build ./...` and `git diff --check` pass; no tests, DB, Worker/provider operation, or scenario ran. Six REQ-24 rows remain partial/not_run; all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-252-req24-replay-auth-guard/verification.md.

## Previous continuation pointer (Slice 251)

Memory-revocation overlay persistence now completes the required parent-directory sync before adding an existing or newly linked disk record to the in-memory fast path. If the sync fails, a later retry rechecks the disk entry instead of returning success from an unverified cache entry. `go build ./...` and `git diff --check` passed. The six REQ-15 rows remain partial/not_run; all 232 frozen scenarios remain not_run. No tests, database, Worker/provider operation, memory correction, restore, or scenario ran. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-251-req15-overlay-fsync-retry/verification.md.

## Previous continuation pointer (Slice 250)

The fixed team-coverage validator now requires exact canonical JSON key names at the top level and for each assignment, preventing server/frontend disagreement on case-variant fields. `go build ./...` and `git diff --check` pass. No tests, database, owner action, Worker/provider operation, or scenario ran. REQ-02 remains partial/not_run, and all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-250-req02-exact-json-fields/verification.md.

## Previous continuation pointer (Slice 249)

Provider terminal telemetry now records the count of observed Codex thread/tokenUsage/updated events. With no usage update it stores token_usage as null and quality as unavailable; when updates arrive it records the latest protocol-reported usage and observation scope. Missing telemetry can no longer look like confirmed zero usage. Workbench money remains unavailable; no dollar estimate, billing liability, or hidden retry count is inferred. Go build and diff check pass; no tests, DB, Worker, provider or scenario ran. REQ-16 rows remain partial/not_run and all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-249-req16-token-usage-quality/verification.md.

## Previous continuation pointer (Slice 248)

Capability rebind after revocation now requires all current WorkerSessions for the Employee to be stopped; MCP capability rebind also requires no dispatching call. If the legacy revoke inventory is incomplete, an existing matching acknowledged_unresolved owner review is required, but that review does not establish quiescence. Exact host stop/restart qualification remains open. Go build and diff check pass; no tests, DB, Worker, provider or scenario ran. REQ-14 rows remain partial/not_run and all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-248-req14-rebind-revocation-fence/verification.md.

## Previous continuation pointer (Slice 247)

Product-provider successors now require the exact prior persisted model profile at both admission and pre-reservation revalidation. The first session and same-profile stopped-session path with owner-reviewed memory revalidation remain unchanged. Cross-profile succession stays denied until an owner-approved transition contract exists; E-HANDOVER thresholds, billing, provider and model qualification remain open. Go build and diff check pass; no tests, DB, Worker, provider or scenario ran. REQ-27/32 rows remain partial/not_run and all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-247-req27-successor-profile/verification.md.

Historical Slice246-era snapshot, superseded by the Slice268 current continuation pointer above: Slice246 propagates owner logout and protected-response invalidation to other same-origin tabs using a non-sensitive marker, with a storage-event fallback. Browser/WebView and remote authentication qualification remain open. Evidence is in `evidence/development/r1-r3-implementation-validation-20261005-slice-246-req25-cross-tab-owner-invalidation/verification.md`. The six REQ-25 rows remain `partial/not_run`; no requirement or scenario status changed. The local checkout is synchronized with `origin/main`; Slice240 implementation commit `b592aa4f839058e53663dac8315a96bb32c0ff2d` is published there; the command-line Git credential is unavailable, but the connected GitHub write integration can push; the migration source at that checkpoint was Schema 108; the database observation then recorded (Slice239) was Schema 108 with zero Companies, Missions and WorkerSessions. Read `docs/implementation/NEXT_SLICE.md` for the current continuation and `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md` for the finite approved scope. Recent stages 167–207 advance local REQ-02/13/14/23/25/29/36/39 implementation and response recovery, add Windows signing integration, and introduce a fake-only logical private Task file tree with immutable draft snapshots. Slice 209 confirms that this tree does not create a shared external writer for REQ-26. Slice 210 restores same-lease recovery when the frozen workspace read fails. Slice 211 adds audited revocation for draft snapshots, while host/shared roots and retention reclamation remain open. Slice 212 refreshes CAP-01–06 crosswalk evidence, Slices 213–214 fence Artifact projections to deliverable kind, and Slice 215 refreshes CAP-06 evidence references only. Slice 216 rejects a tree-backed takeover unless its single private `workspace.txt` matches the legacy frozen workspace at grant and return; Slice 217 links that evidence to all eight REQ-39 scenarios without changing execution status. Slice 218 fences Handover assembly against mixed Company-sequence reads. Slice 222 implements bounded manifest-bound multi-file handback; Slice 223 traces successor delivery and exposes its mapping in the Workbench. Live qualification and frozen scenarios remain open. These do not complete the corresponding frozen qualifications. Items requiring owner decisions, real WorkerSessions, provider accounts, publisher certificates, Windows/Linux qualification hosts, browser evidence, or R3 domain evidence remain open as recorded in the coverage ledger. Do not assume an active WorkerSession; perform Worker work only when a current database read confirms an already-active session. Do not create or start one. The user requires each completed stage to be committed and pushed. Slice 225 reconciles stale REQ-13 cursor claims with Schema 102/Slice 173 and links the evidence to all nine mapped scenarios; quota readiness and a global slot cap remain open. Slice 226 rebuilt the local Termux environment through Schema 106 after archiving the previous empty database whose Schema 102 migration digest failed source verification. Slice 227 clears protected Workbench cache and local component state after API denial/logout; UI-09 remains unqualified. Slice 228 audited the Planning impact gap, and Slice 229 implements an immutable, basis-bound Planning assessment behind isolated fake-only @12. Slice230 restored the local development services; the latest read-only database confirmation in Slice239 reported Schema 108 and zero Companies, Missions and WorkerSessions. Source and evidence pointers are in `NEXT_SLICE.md`. This old Slice246-era continuation instruction is superseded by the Slice268 current continuation pointer above. The user requires each completed stage to be committed and pushed; local Git CLI credentials are absent, so use the connected GitHub write integration when publishing future slices.

## Previous completed slice (254, REQ-29 legacy workspace/tree writer consistency)

The legacy workspace replacement path now validates an existing private tree root under lock, checks its Task owner/Mission and current writer session/epoch, enforces tree bounds, and atomically mirrors `formatter.go` with the legacy workspace row. Submission fails closed unless the tree has one formatter file whose digest and source revision match that row. The six CAP-01–06 rows remain partial/not_run; all 232 frozen scenarios remain not_run. `go build ./...` and `git diff --check` passed. No tests, database, Worker/provider operation, host qualification, or scenario ran. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-254-req29-legacy-tree-writer/verification.md.

## Previous completed slice (253, REQ-31/34 exact stdio MCP manifest fields)

Stdio MCP package manifests now reject unknown/case-variant top-level keys before decoding; literal duplicate keys are still rejected and `args` stays optional. `go build ./...` and `git diff --check` pass. No tests, DB, package import, Worker/provider operation, or scenario ran. The 18 mapped REQ-31/34 rows remain partial/not_run; all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-253-req31-34-mcp-manifest-fields/verification.md.

## Previous completed slice (252, REQ-24 receipt replay authorization guard)

The receipt idempotency path after advisory-lock contention now revalidates the runtime lease and session/Employee epoch in a fresh transaction before returning a matching receipt. `go build ./...` and `git diff --check` pass; no tests, DB, Worker/provider operation, or scenario ran. Six REQ-24 rows remain partial/not_run; all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-252-req24-replay-auth-guard/verification.md.

## Previous completed slice (251, REQ-15 memory revocation overlay durability retry)

Memory-revocation overlay persistence now completes the required parent-directory sync before adding an existing or newly linked disk record to the in-memory fast path. If the sync fails, a later retry rechecks the disk entry instead of returning success from an unverified cache entry. `go build ./...` and `git diff --check` passed. The six REQ-15 rows remain partial/not_run; all 232 frozen scenarios remain not_run. No tests, database, Worker/provider operation, memory correction, restore, or scenario ran. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-251-req15-overlay-fsync-retry/verification.md.

## Previous completed slice (250, REQ-02 exact team-coverage JSON fields)

The fixed team-coverage validator now requires exact canonical JSON keys at the top level and for each assignment, preventing server/frontend disagreement on case-variant fields. `go build ./...` and `git diff --check` pass. No tests, database, owner action, Worker/provider operation, or scenario ran. The seven REQ-02 rows remain `partial/not_run`; all 232 scenarios remain `not_run`. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-250-req02-exact-json-fields/verification.md.

## Previous completed slice (249, REQ-16 token usage quality)

Provider terminal telemetry now records the count of observed Codex thread/tokenUsage/updated events. With no usage update it stores token_usage as null and quality as unavailable; when updates arrive it records the latest protocol-reported usage and observation scope. Missing telemetry can no longer look like confirmed zero usage. Workbench money remains unavailable; no dollar estimate, billing liability, or hidden retry count is inferred. Go build and diff check pass; no tests, DB, Worker, provider or scenario ran. REQ-16 rows remain partial/not_run and all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-249-req16-token-usage-quality/verification.md.

## Previous completed slice (248, REQ-14 capability rebind fence)

Capability rebind after revocation now requires all current WorkerSessions for the Employee to be stopped; MCP capability rebind also requires no dispatching call. If the legacy revoke inventory is incomplete, an existing matching acknowledged_unresolved owner review is required, but that review does not establish quiescence. Exact host stop/restart qualification remains open. Go build and diff check pass; no tests, DB, Worker, provider or scenario ran. REQ-14 rows remain partial/not_run and all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-248-req14-rebind-revocation-fence/verification.md.

## Previous completed slice (247, REQ-27 successor profile continuity)

Product-provider successors now require the exact prior persisted model profile at both admission and pre-reservation revalidation. The first session and same-profile stopped-session path with owner-reviewed memory revalidation remain unchanged. Cross-profile succession stays denied until an owner-approved transition contract exists; E-HANDOVER thresholds, billing, provider and model qualification remain open. Go build and diff check pass; no tests, DB, Worker, provider or scenario ran. REQ-27/32 rows remain partial/not_run and all 232 scenarios remain not_run. Evidence: evidence/development/r1-r3-implementation-validation-20261005-slice-247-req27-successor-profile/verification.md.

## Previous completed slice (246, REQ-25 cross-tab owner invalidation)

Owner logout and protected 401/403 invalidation now notify other same-origin tabs to clear protected views. Messages contain only an invalidation marker and are not rebroadcast; the current tab still clears if cross-tab messaging fails. This reduces stale-cache exposure but does not qualify browser, WebView or remote auth. Six REQ-25 rows remain `partial/not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-246-req25-cross-tab-owner-invalidation/verification.md`.

## Previous completed slice (245, REQ-15 memory overlay permissions)

Memory revocation overlay startup and runtime persistence now share a Unix Company-directory permission check; runtime writes fail closed after permissions broaden. Restore qualification remains open. See `evidence/development/r1-r3-implementation-validation-20261005-slice-245-memory-overlay-runtime-permissions/verification.md`.

## Previous completed slice (244, REQ-13 company cursor fairness)

The dispatcher candidate SQL now orders by Company ID before schedule age, matching the persisted cursor around wrap and preventing Company starvation. The owner-backed global slot cap and authoritative provider quota source remain open. See `evidence/development/r1-r3-implementation-validation-20261005-slice-244-req13-company-cursor-fairness/verification.md`.

## Previous completed slice (243, REQ-29 legacy workspace tree bounds)

The legacy `workspace_replace` compatibility path now shares the logical file-tree bounds and prefix-conflict validator under the root lock and active WorkerSession/epoch fence. CAP and host-mount qualification remain open. See `evidence/development/r1-r3-implementation-validation-20261005-slice-243-req29-legacy-tree-invariants/verification.md`.

## Previous completed slice (242, REQ-02 fixed-team draft semantic guard)

The embedded fixed-team draft now has strict semantic validation; Company creation and acknowledgment reject drift without granting Task admission or execution. All seven REQ-02 rows remain `partial/not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-242-req02-draft-semantic-guard/verification.md`.

## Previous completed slice (241, qualification handoff audit anchor)

Updated `OPEN_QUALIFICATION_BLOCKERS.md` to use the current Slice240 publication-pointer head and date its host/database observations by their source slices. No requirement or scenario status changed. See `evidence/development/r1-r3-implementation-validation-20261005-slice-241-handoff-audit/verification.md`.

## Previous completed slice (240, fixed-role company setup UI)

The New Company wizard shows canonical role names as read-only values bound to fixed Employee IDs, and final review lists all ID-to-role pairs beside the task coverage draft. Display names and model profiles remain editable. Frontend production build and `git diff --check` pass with the existing large-chunk advisory. The Vite frontend returns HTTP 200. No tests, Company creation, owner acknowledgment, database writes, Worker/provider operations or frozen scenarios ran. The seven REQ-02 rows remain `partial/not_run`; UI-01 and all frozen scenarios remain `not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-240-fixed-role-setup-ui/verification.md`.

## Previous completed slice (239, REQ-02 fixed roles from Company creation)

The fixed `TEAM_COVERAGE.json` role mapping now applies from Company creation onward. Creation accepts only the canonical fixed Employee ID/role pairs; existing-company acknowledgment verifies the persisted enabled roster in the same company transaction; all Company updates deny role changes whether or not acknowledgment exists. Company detail and switcher projections report confirmation only when the current matrix digest and canonical persisted roles both match. Qualifications remain unverified and execution disabled. Go packages and rebuilt CLI pass; the local backend is ready and the frontend returns HTTP 200. Read-only PostgreSQL reports Schema 108 with zero Companies, Missions, and WorkerSessions. No tests, owner acknowledgment, database writes, Worker/provider operations or frozen scenarios ran; all 232 scenarios remain `not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-239-req02-fixed-role-contract/verification.md`.

## Previous completed slice (238, REQ-02 confirmed-role immutability)

After exact current-matrix owner acknowledgment, the Company update transaction now rejects changed enabled-roster membership or fixed role names, as required by `role_changes_at_runtime=false`. Company name, workspace path, employee display names, and model profiles remain editable. The guard checks the latest acknowledgment digest against the currently embedded matrix; qualification stays unverified and execution remains disabled. Go packages and rebuilt CLI pass; the local backend is ready and frontend returns HTTP 200. Read-only PostgreSQL confirms Schema 108 and zero Companies, Missions, and WorkerSessions, then rolls back. No owner acknowledgment, tests, database writes, Worker/provider operations or frozen scenarios ran; all 232 scenarios remain `not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-238-req02-confirmed-role-immutability/verification.md`. Published on `origin/main` as `7406d45aac38b7bf380ecb4e82db2ac137940feb`; the local main branch is aligned.

## Previous completed slice (237, REQ-02 owner team-coverage acknowledgment)

The Company setup wizard and existing Company directory both allow explicit installation-owner acknowledgment of the exact embedded fixed-team matrix. The existing-company review reveals all seven canonical assignments and their unverified state; the server requires the exact digest, owner authentication and CSRF protection, and stores the event atomically with the Company write. No role is qualified and execution remains disabled. Go and frontend production builds plus `git diff --check` pass. Read-only local database evidence reports Schema 108 with zero Companies, Missions or active WorkerSessions. No owner confirmation, database write, Worker/provider operation or frozen scenario ran; the seven REQ-02 rows remain `partial/not_run` and all 232 scenarios remain `not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-237-req02-owner-team-coverage-acknowledgment/verification.md`. Slice236 and Slice237 are published as separate commits `56b69a51ea93b7226056bcb508912453365b9690` and `576ebb7476da17e1e6bcdfdb2dd57443c601beab`; local and remote `main` are aligned.

## Previous completed slice (236, REQ-02 fixed team coverage review)

The New Company wizard displays all seven canonical task assignments, checkers, acceptance paths and unverified qualifications. This visibility change did not record approval. See `evidence/development/r1-r3-implementation-validation-20261005-slice-236-req02-team-coverage-review/verification.md`.
## Previous completed slice (235, open qualification dependency audit)

Added a requirement-by-requirement matrix for all 17 open software requirement IDs, with the owner decision, active session, provider/account, Windows/Linux host, or frozen evidence required to resume. Current Android/Termux and Schema 108 observations are separated from external resource availability that remains unknown. No requirement or scenario disposition changed; all 232 scenarios remain not_run. See docs/implementation/OPEN_QUALIFICATION_BLOCKERS.md and evidence/development/r1-r3-implementation-validation-20261005-slice-235-open-qualification-blockers/verification.md.

## Previous completed slice (234, REQ-36 environment preparation retry identity)

Ensure Environment now retains the opaque request ID in tab-scoped session storage by Company and revision, so an ambiguous response can be retried across same-tab route remounts and reloads. Validated success clears the entry. Frontend production build passes; local Schema 108 has zero active WorkerSessions. No preparation request, Worker/provider operation, or frozen scenario ran. UI-42 and WF-08 remain implemented/not_run. See evidence/development/r1-r3-implementation-validation-20261005-slice-234-req36-ensure-retry-continuity/verification.md.

## Previous completed slice (233, REQ-39 owner review state in Workbench)

The Workbench now displays the assessment SHA from consideration/application events and blocks application when the latest current receipt differs from the owner's last review. Go and frontend production builds pass; local Schema 108 services are healthy with zero WorkerSessions. No tests, Worker/provider operation or frozen scenario ran. All eight REQ-39 rows remain `partial/not_run`; all 232 frozen scenarios remain `not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-233-req39-review-state-ui/verification.md`.

## Previous completed slice (232, REQ-39 apply-time Planning assessment fence)

Final Mission change application now requires the current Planning assessment to match the exact assessment digest in the latest owner consideration event. It rechecks stale-basis and high/uncertain-risk block conditions inside the apply transaction, so reassessment after Mission resume requires renewed owner consideration. Go build and rebuilt CLI pass; local Schema 108 services are healthy and there are zero WorkerSessions. No tests, Worker/provider operation or frozen scenario ran. REQ-39 rows remain `partial/not_run`; all 232 frozen scenarios remain `not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-232-req39-apply-assessment-fence/verification.md`.

## Previous completed slice (231, REQ-39 Planning context snapshot consistency)

The Planning request-read tool now selects the unique open request and derives its current impact and assessment basis inside one read-only repeatable-read transaction. A stale requirements/input basis returns a conflict. Go build and rebuilt CLI pass; local Schema 108 services are healthy and there are zero WorkerSessions. No tests, Worker/provider operation or frozen scenario ran. The eight REQ-39 rows cite this evidence and remain `partial/not_run`; all 232 frozen scenarios remain `not_run`. See `evidence/development/r1-r3-implementation-validation-20261005-slice-231-req39-planning-context-snapshot/verification.md`.

## Previous completed slice (230, local development stack recovery)

Restarted the local Termux PostgreSQL, deterministic backend and Vite frontend. PostgreSQL reports Schema 108 and the database has zero Companies, Missions and WorkerSessions; backend health and the Vite root return HTTP 200. No migration, test, Worker/provider activity or frozen scenario ran. All R1–R3 dispositions remain unchanged. See `evidence/development/r1-r3-implementation-validation-20261005-slice-230-local-dev-stack-recovery/verification.md`.

## Previous completed slice (229, REQ-39 Planning impact assessment; Schema 108)

Formal change requests now store bounded immutable impact assessments tied to an active database-confirmed Planning WorkerSession and exact request basis. Owner consideration requires the current assessment digest; high-risk or uncertain assessments require previous results blocked. The isolated opt-in Fake @12 surface leaves real-provider @4 unchanged. Go and frontend production builds pass, Schema 108 is applied, and the rebuilt deterministic backend is healthy. The database has zero WorkerSessions, so no Worker or assessment ran; all eight REQ-39 scenarios remain `partial/not_run` and all 232 frozen states remain `not_run`. Evidence: `evidence/development/r1-r3-implementation-validation-20261005-slice-229-req39-planning-assessment/verification.md`.

## Previous completed slice (227, REQ-21 protected-view invalidation)

Protected Workbench API responses with HTTP 401/403 now invalidate the in-memory React Query cache and remount the active route, clearing component-local details and editors. Denied endpoints are deduplicated until a successful response to the same route, preventing repeated cache-reset loops. A terminally closed protected EventSource triggers the same invalidation; successful installation-owner logout clears the shared cache as well. Frontend production build and `git diff --check` pass. UI-09 stays `partial/not_run`; no tests, auth/logout flow, permission-revocation operation, Worker/provider activity or frozen scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-227-req21-auth-view-invalidation/verification.md`.

## Previous completed slice (226, Termux local development environment)

Rebuilt the local CLI and migrated the Termux PostgreSQL development database to Schema 106. The previous database was at Schema 102 with a recorded migration digest that did not match the checked-in migration; the migration guard correctly refused to continue. A full 775 KB custom-format dump is preserved at `.runtime/termux-pg/backups/polis_r0_termux_schema102-pre-reset-20261005.dump`. The prior database contained zero Companies, Missions or WorkerSessions. After the backup, the empty schema was reset and the standard migration command applied through 106; all 106 migration hashes and evidence rows verify. Backend health returns ready at `127.0.0.1:8080/healthz`; Vite is serving `http://127.0.0.1:4173/` with HTTP 200. No tests, Worker/provider activity or scenarios ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-226-termux-local-dev-env/verification.md`.

## Previous completed slice (225, REQ-13 fairness coverage reconciliation)

Reconciled stale REQ-13 descriptions with Schema 102/Slice 173: candidate selection now uses a database-locked shared Company cursor plus short-lived per-Task claims, replacing the earlier process-local cursor. An owner-backed global concurrency cap and authoritative provider quota readiness/recovery are still absent; quota remains fail-closed and the dispatcher remains opt-in Fake @7. Added Slice 225 evidence to all nine mapped scenarios; all remain `partial` / `not_run`, with 232 scenario states still `not_run`. JSON validation and diff checks pass; no tests, database, Worker/provider activity, or scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-225-req13-fairness-reconciliation/verification.md`.

## Previous completed slice (224, REQ-23 closeout coverage reconciliation)

The coverage row still said `closing`, `ended_not_met`, responsibility settlement and owner controls were open, though Schema 101 and Slices 169–170 implemented them. The row now reflects the implemented closeout path and retains restart/recovery and FT-57–60/72 qualification as open. Slice 224 evidence is linked from all seven mapped scenarios; their disposition/status remain `partial` / `not_run`, and all 232 scenario states remain `not_run`. JSON validation and diff checks pass; no tests, database, closeout operation, Worker/provider action or scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-224-req23-closeout-reconciliation/verification.md`.

## Previous completed slice (223, REQ-39 successor directory-input trace)

Source review confirmed a returned directory MissionInput is included in formal change impact, cloned with its archive source kind/digest into the successor Mission, and bound into successor Task input delivery. The Workbench now displays the count of human-returned snapshots separately. All eight REQ-39 crosswalk rows cite Slices 222/223 evidence and remain `partial` / `not_run`; all 232 frozen scenario states remain `not_run`. Frontend build, JSON crosswalk validation and diff checks pass. No tests, database, change request, Task, Worker/provider action or scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-223-req39-successor-delivery/verification.md`.

## Previous completed slice (222, REQ-39 bounded directory handback)

Tree-bound takeover leases now accept complete bounded directory snapshots and persist them as immutable MissionInputs with manifest-bound change summaries and exact-request replay. Schema 106 expands the receipt byte ceiling for the existing archive format. The Workbench supports local file edits, additions, deletions and same-payload retries. Go/frontend builds and migration hash/diff checks pass. No tests, database migration, lease command, Worker/provider action or frozen scenario ran. REQ-39 remains partial. See `evidence/development/r1-r3-implementation-validation-20261005-slice-222-req39-directory-snapshot-handback/verification.md`.

## Previous completed slice (218, REQ-27 handover sequence fence)

`work_current`, `context_read`, and `workspace_read` now recheck `company_seq` after reading the base Handover and optional direct-message, shared Artifact, and MCP projections. They retry the composed read at most three times and return a retryable conflict if concurrent writes keep changing the Company. Go build and diff checks pass. No tests, database operation, Worker/provider activity or frozen scenario ran. All eight REQ-27 scenarios remain `partial` / `not_run`; E-HANDOVER behavioral qualification remains open. See `evidence/development/r1-r3-implementation-validation-20261005-slice-218-req27-handover-sequence-fence/verification.md`.

## Previous completed slice (217, REQ-39 evidence crosswalk refresh)

Slice 216's legacy/tree takeover baseline evidence is now linked from all eight REQ-39 frozen scenarios. `PP-09`, `UI-45`, `WF-21`–`WF-25`, and `WF-36` remain `partial` / `not_run`; no frozen execution status or disposition changed. JSON parsing and diff checks pass. No tests, implementation code, database, Worker/provider activity or scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-217-req39-evidence-crosswalk/verification.md`.

## Previous completed slice (216, REQ-39 legacy/tree baseline fence)

Task takeover grant and snapshot handback compare any existing Schema 103 tree against the frozen legacy workspace. Only one same-Mission private `workspace.txt` with matching digest and source revision is accepted; multi-file, `formatter.go`, or divergent roots conflict. Tasks without a tree retain the legacy bounded behavior. `go build ./...` and `git diff --check` pass. No tests, database operation, lease command, Worker/provider activity or scenario ran. Full multi-file manifest binding remains open. See `evidence/development/r1-r3-implementation-validation-20261005-slice-216-req39-tree-baseline-fence/verification.md`.

## Previous completed slice (215, CAP-06 evidence pointer refresh)

CAP-06 now cites Slice 213/214's Artifact-delivery and Mission-projection fence evidence. It remains `partial` / `not_run`; all scenario execution states, dispositions and counts are unchanged. JSON validation and `git diff --check` pass. No tests, code, database, Worker/provider activity or scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-215-cap06-evidence-refresh/verification.md`.

## Previous completed slice (214, REQ-29 Mission Artifact projection fence)

The generated sqlc `ListArtifacts` query and its `db/queries.sql` source now filter to `artifact_kind='deliverable'`. This prevents workspace snapshots from appearing as ordinary Artifacts in recovery and peer-state Mission snapshots. `go build ./...` and `git diff --check` pass. No tests, database operation, Worker/provider activity or frozen scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-214-req29-mission-artifact-projection-fence/verification.md`.

## Previous completed slice (213, REQ-29 Artifact delivery kind fence)

Workbench's Artifact delivery-manifest lookup now explicitly requires `artifact_kind='deliverable'`, matching the overview/detail read boundary and preventing workspace snapshots from appearing as downloadable deliverables if a qualification row is ever attached. `go build ./...` and `git diff --check` pass. No tests, database, Worker/provider or frozen scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-213-req29-delivery-kind-fence/verification.md`.

## Previous completed slice (212, REQ-29 frozen traceability refresh)

The requirement-to-scenario crosswalk now cites Slice 211's revocation evidence for CAP-01–06 while retaining the exact `partial` dispositions and `not_run` execution states. All 232 scenario records and disposition counts are unchanged. JSON validation and `git diff --check` pass; no code, migration, test, Worker/provider action or scenario ran. See `evidence/development/r1-r3-implementation-validation-20261005-slice-212-req29-traceability-refresh/verification.md`.

## Previous completed slice (211, REQ-29 draft workspace snapshot revocation; Schema 104)

Ready draft workspace snapshots now have immutable, audited revocation receipts, bound to the active source WorkerSession/epoch. Receipt insertion and the Artifact state change are atomic; future manifest/file reads fail, while prior reads and retained CAS bytes remain intact. Deliverable projections and startup recovery exclude revoked snapshots. The optional fake-only @11 surface was added without changing @10 or real-provider @4 authorization. Go build, all migration hashes and diff checks pass. No tests, migration application, Worker/provider session or frozen scenario ran. REQ-29 remains partial. See `evidence/development/r1-r3-implementation-validation-20261005-slice-211-req29-workspace-snapshot-revocation/verification.md`.

## Previous completed slice (210, REQ-39 frozen workspace read retry)

Takeover Workbench marks the lease base loaded only after validating the frozen digest, revision and supported single-file shape. On failure, an explicit retry reads against the same lease; no extra lease command is needed. Frontend production build and `git diff --check` pass; tests, lease/snapshot commands, Worker/provider activity and frozen scenarios did not run. Multi-file handback remains open pending a frozen tree manifest and bounded return contract. See `evidence/development/r1-r3-implementation-validation-20261005-slice-210-req39-frozen-workspace-read-retry/verification.md`.

## Previous completed slice (209, REQ-26 shared-write audit refresh; Schema 103)

The post-Slice-208 source review confirms that private Task CAS writes are not shared branch/deployment/publish writes. No ResourceKey model or shared writer exists to bind; `companies.workspace_root` is not a runtime path authorization boundary. REQ-26 remains partial and FT-66/FT-70 remain `not_run`. This was source-only; no code, migration, Worker/provider action, or external write ran. See `docs/implementation/REQ26_RESOURCE_BINDING_AUDIT.md` and `evidence/development/r1-r3-implementation-validation-20261004-slice-209-req26-resource-binding-audit/verification.md`.

## Previous completed slice (208, REQ-29 logical private Task tree; Schema 103)

Schema 103 adds a CAS-backed private Task file tree and immutable `draft_not_accepted` workspace snapshots. The opt-in fake-only @10 surface exposes bounded list/search/read/write/delete and same-Mission exact-ID snapshot reads; writes are fenced by the current WorkerSession/epoch and root revision. `formatter.go` remains compatible with existing single-file validation and delivery, and the legacy acceptance path denies multi-file trees. Snapshot Artifacts do not enter existing deliverable or memory dependency flows. `go build ./...`, migration hash checks and `git diff --check` pass. No tests, DB migration, Worker/provider action or CAP scenario ran. REQ-29 remains partial; this is a logical CAS workspace, not a host mount, and company/group shared roots and CAP-01–06 qualification remain open. See `docs/implementation/REQ29_SHARED_FILE_ACCESS_AUDIT.md` and `evidence/development/r1-r3-implementation-validation-20261004-slice-208-req29-workspace-tree/verification.md`.

## Previous completed slice (207, REQ-39 pending snapshot safety)

Workbench disables and rejects takeover-lease release while a snapshot return has an unresolved exact-payload retry, retaining the content, frozen digest/revision and request ID until the result settles. This avoids invalidating the same-operation retry path with a competing release. Frontend production build and `git diff --check` pass; no test or lease/snapshot/Worker/provider command ran. See `evidence/development/r1-r3-implementation-validation-20261004-slice-207-req39-pending-snapshot-safety/verification.md`.

## Previous completed slice (206, handoff pointer synchronization)

Synchronized the repository's top-level `AGENTS.md` and cloud handoff pointer with the current Slice 205 ledger and latest migration source Schema 102. The coverage ledger remains the authoritative list of open software and external qualification work. Updated old handoff statements to label completed slices as historical and explicitly mark the pre-Slice-151 REQ-29 gap statement as superseded. No code, database, tests, Worker/provider action, or external operation was performed. See `evidence/development/r1-r3-implementation-validation-20261004-slice-206-handoff-pointer-synchronization/verification.md`.

## Historical completed slice (205, REQ-29 audit reconciliation)

The current source audit now correctly distinguishes Slice 151's fake-only @8 same-Mission ready Artifact reads from the still-unmodeled arbitrary directory tree and mutable current-workspace snapshots. Coverage and continuation notes preserve Slice 150 as historical. No code, database, tests, Worker/provider action, or external operation was performed. See `evidence/development/r1-r3-implementation-validation-20261004-slice-205-req29-audit-reconciliation/verification.md`.

## Previous completed slice (204, Windows signing integration)

The signed Windows Tauri packaging path is opt-in and fails closed unless a valid current-user publisher certificate, timestamp URL, SignTool, and packaged PostgreSQL runtime are available. Frontend production build and diff checks passed; no Windows package/signing action was run. See `evidence/development/r1-r3-implementation-validation-20261004-slice-204-windows-signing-integration/verification.md`.

## Historical completed slice (166, REQ-13 quota readiness/recovery boundary audit)

Source review confirms `waiting_quota` is sticky and admission-safe, but no production Go path writes/releases it and no authoritative provider quota readiness source exists. `provider_quota_exhausted` is only a notification reason. Automatic recovery has no trusted release condition to implement; the earlier process-local cursor finding was superseded by Schema 102/Slice 173, which added a database-persisted shared cursor and per-Task claims. No code/schema change, tests, Worker or provider activity ran. REQ-13 remains open. See Slice 166 evidence.

## Historical completed slice (165, REQ-14 owner-reviewed unresolved disposition)

Schema 100 adds immutable installation-owner review receipts for old incomplete capability revocations. The only disposition is `acknowledged_unresolved`; it records that historical completeness remains unknown, requires a validated owner session plus CSRF, and rechecks the active incomplete target under the Company write guard. The projection remains `sessionInventoryComplete=false` and `quiesced=false`; the action neither reconstructs missing sessions nor stops Workers. Local Termux migrated to Schema 100 and has Company=0, owner=0, WorkerSession=0. Native/Windows Go builds, frontend build, migration hash verification and diff check pass; tests and owner/Worker/provider activity did not run. REQ-14 remains open for any applicable owner review and runtime qualification. See Slice 165 evidence.

## Historical completed slice (164, REQ-14 complete revocation-inventory receipt)

The prior projection could treat a pre-Schema-73 revocation with no usage rows as quiesced even though it could not prove whether an unused but bound WorkerSession existed at revoke time. Schema 99 adds an immutable completion receipt written in the same revoke transaction, including for an exact snapshot containing zero sessions. The projection prefers a receipt or a nonempty Schema-73 snapshot; an older zero-row revocation without either remains `sessionInventoryComplete=false` and cannot report quiesced. Workbench now labels that state “需复核”. The local development database was migrated to Schema 99. Linux and Windows/amd64 Go builds, frontend production build and `git diff --check` pass. No tests or Worker activity ran. REQ-14 remains open for legacy review and runtime qualification. See Slice 164 evidence.

## Historical completed slice (163, REQ-14 restart retry for revoked Worker stop)

Capability revocation snapshots the affected live/use-recorded WorkerSessions in the revocation transaction. The background coordinator drains those durable snapshots and retries; after restart, the adapter delegates to exact-session host reconciliation. Kernel recovery marks non-stopped sessions `reconcile_required`, but the Windows exact-session path previously accepted only `restoring`, so repeated retries could never progress after a startup host reconciliation failure. It now accepts `reconcile_required` too and retains exact session, containment-profile, host-tree proof and database-state checks. Linux already accepted this state. Native and Windows/amd64 command builds plus `git diff --check` pass. No tests or Worker stop were run, and CAP/FT qualification remains open. See Slice 163 evidence.

## Historical completed slice (162, REQ-13 dispatch fairness/quota/WorkerSession boundary)

Source audit confirms that automatic dispatch is opt-in and only accepts the exact zero-egress Fake @7 surface. It selects active-company/active-Mission ready work from `wake_pending`, advances one company cursor per 30-second cycle, and leaves session creation to the normal database admission transaction. That transaction locks the Employee schedule, rejects `paused` and `waiting_quota`, rejects a live session, and creates a `restoring` WorkerSession. The adapter validates and activates that persisted session before starting the run loop. No production quota recovery writer, owner-backed global slot cap, or provider readiness source was found; the cursor portion of this older finding was superseded by Slice 173. No Worker was started; the local database has no active WorkerSession. REQ-13 remains open. See Slice 162 evidence.

## Historical completed slice (161, REQ-15 FT-41 old-backup memory revocation audit)

Desktop uses a stable tombstone root outside generation CAS directories. Kernel loads tombstones before DB recovery and applies them to the restored database before the runtime becomes available. This blocks logical memory access after restoring an older generation on the same data root, but does not erase old CAS/DB bytes. Source audit only; FT-41 and Desktop restore remain `not_run`. See Slice 161 evidence.

## Historical completed slice (160, REQ-25 restored installation-owner session revocation)

Recovery holds both the Kernel control-plane lock and owner-auth lock through completion. Schema 97+ restores revoke every unrevoked installation-owner token digest and add immutable `session_revoked` auth events in the same transaction as MCP binding revocation. The marker requires both gates; old markers can retry safely. Owner credentials remain, so login can be re-established. Build/diff checks pass; restore, browser, WorkerSession and frozen CAP/FT scenarios were not run. See Slice 160 evidence.

## Historical completed slice (159, CAP-31 recovery-generation MCP authorization gate)

Recovery reserves the Kernel control-plane advisory lock across database/CAS restore and finalization. It records deterministic append-only revocations for every currently bound MCP Employee capability before completing the generation marker; staging state allows safe idempotent retry after a partial finalization. Any later execution needs a fresh binding and database-confirmed active WorkerSession. Build and diff checks pass; tests, actual restore, WorkerSession/endpoint and frozen CAP execution were not run. CAP-31 remains `not_run`. See the Slice 159 evidence.

## Historical completed slice (151, REQ-29 same-Mission shared Artifact reads; no migration)

An opt-in exact fake-only @8 Worker surface lists bounded pages of ready candidate/passed Artifacts in the current Company+Mission and reads a selected Artifact ID after database-confirmed active WorkerSession/Task ownership checks and CAS digest/size/UTF-8 verification. Reads append exact version/session provenance and bounded Handover history. @4 remains the real-provider gate and @7 is unchanged. The environment flag is off by default. See the Slice 151 audit and evidence.

## Historical completed slice (150, REQ-29 shared-file access audit; no code/schema change)

Task input manifests deliver immutable Mission inputs and a Worker can read/write its current Task workspace with digest+revision fencing. At Slice 150 no Worker tool listed or read another Task's published Artifact; Slice 151 later added same-Mission ready candidate/passed Artifact reads on fake-only @8. Arbitrary directory trees and mutable current-workspace snapshots remain unmodeled. Recovery snapshots remain separate. See the updated REQ-29 audit and the original Slice 150/151 evidence.

## Historical completed slice (149, REQ-26 C-RESOURCE audit; no code/schema change)

The source audit found no production ResourceKey model or shared branch/deploy/publish writer. Current Git import is read-only; Task workspaces are per-Company/Task digest metadata; project preparation uses fresh random application-managed workspaces. The Company `workspace_root` setting is not a runtime path boundary. REQ-26 remains open for a future actual shared writer and must not be represented as protected by an inert registry. See the Slice 149 audit and evidence.

## Historical completed slice (148, REQ-15 WorkerSession-bound memory corrections; Schema 98)

Workbench proposal/review requests carry only a WorkerSession ID. Kernel derives Employee and Task from durable session rows, rechecks the active provider session and working Task under the write transaction, preserves exact session/Task provenance with compound foreign keys, and keeps the fixed Planning/Review and independent-review rules. The approval UI explicitly confirms downstream invalidation. Go and frontend builds, checksums, local Schema 98 migration and backend/frontend health checks pass. The main local database still has no owner, Company or correction data; correction-command E2E and frozen scenarios were not run. See the Slice 148 evidence.

## Historical completed slice (146, FT-63 owner-session API integration)

A disposable database verified bootstrap, login, protected read, CSRF denial, logout and revocation. The disposable database was dropped. The main local dev database has no owner credential; the owner must choose it through the Workbench. Browser E2E is not complete. See Slice 146 evidence.

## Historical completed slice (145, PostgreSQL migration compatibility; Schema 97 applied locally)

The local Termux database is now at Schema 97. Historical migration parser gaps in 78/79 were normalized only for Goose's reader, preserving source hashes, and the new PL/pgSQL function in 97 was properly bracketed. Backend health and owner status/session routes are responding. Owner enrollment is left pending until the installation owner chooses a password. See Slice 145 evidence.

## Historical completed slice (144, FT-63 Workbench owner setup/login UI)

The Group navigation now includes a first-owner setup/login/logout page and read-only observed-account table. It hides first-owner enrollment on HTTPS remote Workbench and clarifies that locator fingerprints do not prove billing scope. TypeScript, ESLint and production build pass; database/browser E2E remains open because PostgreSQL is unavailable. See Slice 144 evidence.

## Historical completed slice (143, FT-63 revocable owner sessions; Schema 97)

The backend now issues 12-hour revocable owner sessions, stores token/CSRF digests only, rate-limits failed logins and enforces a session-bound CSRF header for unsafe cookie-authenticated requests. Focused Go verification and build pass; PostgreSQL migration/runtime and browser E2E remain unverified. The owner setup/login UI is the next local stage. See Slice 143 evidence.

## Historical completed slice (142, FT-63 first-owner bootstrap; Schema 96)

The terminal-only command issues a short-lived random setup code; Schema 96 stores only its digest. The local-origin endpoint throttles attempts and atomically creates the single owner password record. Browser sessions, cookies, CSRF and UI login remain open. See Slice 142 evidence.

## Historical completed slice (141, FT-63 bounded owner password hash primitive; no migration)

The owner password helper uses fixed bounded Argon2id parameters, random salts and constant-time comparison. It is not yet wired to owner enrollment/login. See Slice 141 evidence.

## Historical completed slice (140, REQ-16 observed ProviderAccount registry; Schema 95)

The immutable registry backfills existing available account locators and links future observations to WorkerSessions. A token-protected, read-only Workbench endpoint exposes only fingerprints and aggregate counts; the shared desktop token is not the dedicated first-owner login/session design. Financial caps, liability settlement and live account qualification remain open. See Slice 140 evidence.

## Historical completed slice (139, REQ-16 ProviderAccount billing-scope audit; no migration)

Official sources distinguish ChatGPT shared credits/allowances, Enterprise token-based USD billing and API Platform organization/project billing. Enterprise Codex Analytics requires a workspace-scoped Admin key. No such key is configured here; live workspace usage/cost validation remains external. Kernel has only Company scope, so shared ProviderAccount mutations need an installation-owner boundary before implementation. See Slice 139 evidence.

## Historical completed slice (138, REQ-16 Codex account locator snapshot; Schema 94)

For ChatGPT auth files, Codex `tokens.account_id` (or the supported ID-token account claim) is hashed and bound immutably to each WorkerSession before execution; unavailable/unsupported cases are explicit. The runtime rejects a changed available account locator after binding. This is the selected Codex account routing scope, not proven billing coverage or a financial budget key. Go build, Schema 1–94 hashes and diff check pass; tests, PostgreSQL migration/runtime and provider execution were not run. See Slice 138 evidence.

## Historical completed slice (137, REQ-16 general Codex auth-principal observation; no migration)

General Codex business runtimes now derive the auth-principal fingerprint when the mounted auth file contains reconstructable issuer/subject claims; otherwise identity remains unavailable. After an identity snapshot is pinned to a WorkerSession, the runtime rechecks an available fingerprint before reservation/start and stops if it changed. This does not prove ProviderAccount billing identity or measure spend. Go build, Schema 1–93 hashes and diff check pass; tests, PostgreSQL runtime and provider execution were not run. See Slice 137 evidence.

## Historical completed slice (136, REQ-16 provider auth identity snapshot; Schema 93)

Schema 93 stores an immutable provider auth-identity snapshot per WorkerSession before execution. The optional versioned runtime capability reports available/unavailable/unsupported; only the existing LIVE_2 Codex path reports a fingerprint. Capture or persistence errors finalize the unstarted WorkerSession. The fingerprint is an auth-principal hint rather than a ProviderAccount ID or financial accounting unit. Go provider/kernel/control/workbench/command build, migration hash checks through 93 and `git diff --check` pass. Tests, PostgreSQL migration/runtime and provider execution were not run. ProviderAccount identity semantics outside LIVE_2 and its request-liability lifecycle remain open. See Slice 136 evidence.

## Historical completed slice (135, REQ-16 ProviderAccount identity/liability binding audit; no migration)

The general provider runtime and execution authorization carry no ProviderAccount identity. The auth identity snapshot is limited to the Codex LIVE_2 credential path and is not exposed to ordinary Worker admission. Durable usage is session-scoped; no account-level request reservation, settlement or unknown-liability ledger exists. Financial ProviderAccount caps remain deferred until those foundations are implemented. No code/schema/build/test/database/provider execution was changed or run; `git diff --check` passes. See Slice 135 evidence.

## Historical completed slice (134, REQ-16 Company closing reserve; Schema 92)

Schema 92 adds append-only Company reserve revisions inside its total protocol-tool-call cap. Only Kernel-created `review` and `peer_review` Tasks can use protected calls; ordinary work preserves remaining reserve during admission, charging, Handover and provider turn clamping. Closing calls burn reserve and Company total atomically. Pending Companies may configure reserve first, but initial cap configuration must cover existing usage plus remaining reserve. Immutable Company denial snapshots include reserve state, and Settings exposes confirmed reasoned updates. Go package/command build, frontend production build, hashes through 92 and `git diff --check` pass; Vite reports the existing 810.97 kB minified bundle advisory. Tests, PostgreSQL migration/runtime, provider execution and live cost measurement were not run. This measures admitted protocol tool calls only. See Slice 134 evidence.

## Historical completed slice (133, REQ-16 Company protocol tool-call cap; Schema 91)

Schema 91 backfills Company usage from durable Task counters and leaves the cap pending until explicit local-owner configuration. Append-only allocation history permits configuration/increases; immutable Company admission/call rejection evidence binds cap, usage and revision. Worker admission checks Company before Mission and accepted protocol tool calls charge Company with Session, Task, ProblemKey and Mission in one transaction. Handover and provider turn clamping use the minimum remaining calls. Settings exposes a no-store projection and confirmed reasoned change form. All Companies need explicit configuration after migration before new admissions/calls proceed. This does not measure model requests, hidden retries, tokens, provider egress or USD. Go package/command build, frontend production build, migration hashes through 91 and diff check pass; the existing Vite >500 kB advisory remains. Tests, PostgreSQL migration/runtime and provider execution were not run. ProviderAccount identity/liability, hidden retries, token/money accounting and FT-42–45 qualification remain open. See Slice 133 evidence.

## Historical completed slice (132, REQ-16 Company/Provider budget identity audit; no migration)

Company ID is stable; `txWrite` locks Company before budget-related row locks; admitted tool calls use idempotency keys derived from session and provider call ID; Task counters support conservative Company usage backfill. This enables an explicitly labeled Company protocol-tool-call cap, not a model/token/USD cap. General WorkerSessions do not bind a ProviderAccount ID, and terminal token observations are not settled provider-account spend. Do not enforce a strict financial ProviderAccount cap until identity and liability accounting exist. See the Slice 132 evidence.

## Historical completed slice (131, REQ-16 Mission closing reserve; Schema 90)

Schema 90 adds append-only Mission reserve revisions inside the total cap. Only Kernel-created `review` and `peer_review` Tasks can consume protected calls. Ordinary work preserves reserve through admission, accepted-call accounting, Handover and provider turn clamping; closing calls consume it while still counting toward Mission total. Pending Missions may set reserve, but the first finite cap must cover usage and unspent reserve. Immutable rejection snapshots include reserve revision/state, and Settings exposes a confirmed reasoned update. `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis`, `npm run build`, all 90 migration hashes and `git diff --check` pass. Tests, PostgreSQL migration/runtime and provider execution were not run. See `docs/implementation/REQ16_MISSION_BUDGET_COMPOSITION.md` and the Slice 131 evidence.

## Historical completed slice (130, REQ-16 explicit Mission tool-call cap; Schema 89)

Schema 89 backfills Mission usage from durable Task counters while leaving caps unset. New Mission creation requires an explicit finite cap; Settings configures pending legacy Missions and permits only total-cap increases through a confirmed, reasoned append-only ledger. Admission and each accepted call serialize Mission → WorkerSession/Task → ProblemKey, incrementing applicable counters atomically. Immutable Mission-level denial snapshots, Handover fields and provider turn clamping use the same cap. `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis`, `npm run build`, all 89 migration hashes and `git diff --check` pass. Tests, PostgreSQL migration/runtime and provider execution were not run. Company/Provider/token/money accounting remain open. See `docs/implementation/REQ16_MISSION_BUDGET_COMPOSITION.md` and the Slice 130 evidence.

## Historical completed slice (129, REQ-16 Mission budget composition and lock-order audit; no migration)

The source-backed Mission cap design and lock-order rationale are recorded in `docs/implementation/REQ16_MISSION_BUDGET_COMPOSITION.md`; Slice 130 implements the cap under that decision.

## Historical completed slice (128, REQ-16 scoped provider retry observation; no migration)

Real Codex Worker usage identifies that the reconnect count covers only app-server `responseStreamDisconnected` events marked `willRetry`. The terminal record stores that scope and `unobserved_provider_retry_count: null`; retries internal to the CLI/service are not exposed or charged. `retry_visibility=limited` remains. This is conservative observability, not complete retry accounting. `go build ./internal/kernel ./internal/control ./cmd/polis`, frontend build, all Schema 88 migration hashes and `git diff --check` pass. Tests and provider execution were not run. Evidence: `evidence/development/r1-r3-implementation-validation-20261003-slice-128-provider-retry-visibility/verification.md`.

## Historical completed slice (127, REQ-16 explicit incomplete budget closeout; Schema 88)

Schema 88 adds an immutable local-owner incomplete Task closeout tied to the latest budget-caused rejection and exact Task, ProblemKey and closing-reserve snapshots. It requires a nonterminal Task with no live WorkerSession and prevents later Worker admission or Task-budget allocation. The Workbench exposes reasoned confirmation and shows the outcome. Together with Schema 86 rejections and Schema 84/87 owner allocations, exhausted work can remain blocked, receive funded recovery subject to normal Worker admission gates, or close explicitly as incomplete. Admission, allocation and closeout serialize on the Task row. Go package/command build, frontend production build and `git diff --check` pass; the frontend reports the existing >500 kB advisory. Tests and PostgreSQL migration/runtime were not run. Evidence: `evidence/development/r1-r3-implementation-validation-20261003-slice-127-task-budget-incomplete-closeout/verification.md`.

Slice 128's scoped provider retry observation is recorded above. Slices 130–131 add Mission tool-call enforcement and its closing reserve; Company cap implementation, ProviderAccount identity/limits, hidden CLI retries, token/money accounting and scenario qualification remain open.

## Objective and boundaries

Continue the user-approved Polis R1–R3 implementation from the frozen v0.4.5 design pack and the live coverage ledger at `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`. Work toward the full approved plan; do not redefine completion around the slices already present. Keep the work finite and tied to the existing REQ/FT/NT/CAP/UI/WF/PP traceability.

Explicit exclusions remain office/3D, dynamic hiring/firing, arbitrary MCP compatibility, and a plugin marketplace. Do not run real employee model turns, real QQ sends, external MCP/GitHub actions, business-account flows, or production actions without separate authorization. Preserve historical R0 evidence and the original design snapshot.

## Source of truth

- GitHub repository: `https://github.com/chyinan/Polis`
- Branch: `main`
- Current code checkout used for this continuation: `/data/data/com.termux/files/home/polis` (Termux)
- Slice101 feature commit: `998f425` (`feat: add fake-only product direct messaging`), followed by review-fix commits `aa0a53e` and `6df7d54`. They add ordered Task row locks, an active-Mission `FOR SHARE` lock, and explicit rejection of an omitted `actionable` field. Read-only re-review found zero remaining findings.
- An older local checkout exists at `D:\Programs\Polis` on `master` with unrelated uncommitted/untracked files. Do not use that checkout as the current source of truth or copy its working-tree contents into `main`.

Clone/pull `main` before continuing. Read this file, `AGENTS.md`, `docs/implementation/NEXT_SLICE.md`, `docs/implementation/PROGRESS.md`, and `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md` first. The coverage ledger is authoritative for the finite remaining scope.

## Just-closed slice

Slice102 adds an opt-in, bounded automatic dispatcher for the fixed product `compat/emp-backend` Task. It selects at most one eligible Task every 30 seconds, rotates by Company ID after each attempt, requires an active Company/Mission, a unique ready Task with validation binding and workspace, no prior WorkerSession, no other live session for the Employee, and `wake_pending` schedule state. WorkerSession admission remains the final authority, and the dispatcher shares the Mission lifecycle lock. It never clears `paused` or `waiting_quota`.

The dispatcher is off by default and requires both `POLIS_AUTO_WORKER_DISPATCH_ENABLED=1` and the exact zero-egress Fake @7 surface. It does not dispatch Routine Tasks, retry a Task after a WorkerSession exists, or use real provider transport. Schema 102/Slice 173 later replaced the process-local cursor with a shared database cursor and short-lived Task claims. Global slot accounting, authoritative quota readiness/recovery, and production dispatch remain open. No schema migration or real Worker/model turn was used.

Slice103 adds the first REQ-14 capability-revocation status projection. The company capability catalog and Workbench show current global/employee Skill/MCP revocations, accepted/effective-for-new-dispatch flags, quiescence, recorded-use WorkerSessions, and MCP call outcomes. The projection rebuilds from durable rows in a repeatable-read transaction, needs no process-local restart state, and uses bounded detail with complete counts and truncation flags. `quiesced` requires every inventoried session to be `stopped` and zero `dispatching` MCP intents; `reconcile_required` remains live and `outcome_unknown` stays visible. Schema remains 72; no Worker stop is initiated.

The projection only inventories sessions with recorded Skill loads or MCP intents; it does not capture a bound-but-unused session at revoke time, stop a Worker, or show superseded historical revocations. Next capture the affected-session set at the revoke linearization point, then implement idempotent stop/restart reconciliation before wiring a management action. Verification: Go command build, frontend production build, and diff check pass; tests were not run. Evidence: `evidence/development/r1-r3-implementation-validation-20261001-slice-103-capability-revocation-projection/verification.md`.

Slice104 adds Schema 73 revoke-time snapshot ledgers. Both global capability revocation and employee unbind capture their affected WorkerSessions inside the guarded transaction; the snapshot stores each session's state and Skill-load count and pins exact matching MCP intent IDs plus their initial statuses. The catalog projection reads these snapshots for new revocations, so later sessions/intents after a rebind do not leak into the old revoke inventory. Pre-Schema-73 revocations retain the event-ledger fallback. This still does not stop Workers. Verification is build-only (Go command/frontend builds and diff check); no tests or PostgreSQL migration execution were run. Evidence: `evidence/development/r1-r3-implementation-validation-20261001-slice-104-revocation-session-snapshot/verification.md`.

Verification on this Termux host:

- `go test ./internal/control ./cmd/polis` passes; `go test ./internal/kernel -run '^$'` compiles the package without running tests.
- `go build ./cmd/...` passes for `android/arm64`; `GOOS=windows GOARCH=amd64 go build ./cmd/...` passes.
- The combined Kernel/control/command test run hits the Termux seccomp denial of `fchmodat2` in the existing `TestUnixParentDirectorySyncUnsupportedSentinelRemainsFatal`; the dedicated PostgreSQL suite was not run because the bundled x86_64 `.tools/pg` runtime is absent. The persistent development database was left untouched.
- `git diff --check` passes. `rtk` is unavailable in this Termux environment.

Evidence: `evidence/development/r1-r3-implementation-validation-20261001-slice-102-auto-worker-dispatch/verification.md`.

Slice101 (below) remains the prior product direct-messaging slice.

Slice101 connects direct messaging to the generic product Worker adapter through the separately versioned 12-tool `polis-product-tool-surface@7`. It provides bounded same-Mission target discovery, direct send, ordered inbox, acknowledgement, application evidence, and candidate-Artifact resolution. The Kernel locks the Mission row `FOR SHARE`, then locks source and target Tasks in stable ID order, then checks the current WorkerSession Task, explicit recipient, fixed employee roster, Mission, and Task states. The product API rejects missing or null `actionable`; `false` explicitly selects FYI. Actionable messages persist the Obligation, work signal, and schedule wake transactionally. FYIs create no Obligation or wake and advance in event order after acknowledgement.

The exact @7 manifest is `82d7b2dbc41ff3dbed56813b3b3adcfad48818fb29653bcd2debf1f280e507eb`; the aggregate schema is 3503 bytes with digest `769f7c9f4afb1c1d0ebfb43037a06d8f2ddb661bc111970c4d199f48f962fd42`. Only the exact zero-egress Fake Runtime purpose/envelope/markers accept @7. The real-provider path remains pinned to @4. Product-provider Task execution is still restricted to `compat/emp-backend`; this slice does not qualify other fixed roles or any real provider interaction. Schema remains 72; no migration was added.

Verification completed:

- `rtk bash scripts/go.sh test ./internal/codex ./internal/kernel ./internal/provider ./internal/control`
- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` on a dedicated disposable PostgreSQL 18 instance through Schema 72, including send/submission and send/pause event-order checks
- `rtk bash scripts/go.sh test ./...`
- `rtk bash scripts/go.sh build ./cmd/...` for Linux amd64
- `rtk bash -lc 'GOOS=windows GOARCH=amd64 ./scripts/go.sh build ./cmd/...'`
- `rtk git diff --check` after the final handoff edit.

Evidence: `evidence/development/r1-r3-implementation-validation-20260930-slice-101-product-direct-worker/verification.md`.

## Historical remaining-work snapshot (superseded by the Slice268 current continuation pointer)

Use the current finite list in `R1_R3_IMPLEMENTATION_COVERAGE.md`; it contains the open software requirements and keeps them separate from owner-, account-, certificate-, and host-dependent qualification. REQ-02/13/14/15/16/23–27/29/30–39 remain partial in the specific ways listed there. Slice 205 reconciled the REQ-29 audit; Slice 204 added a fail-closed signing integration, but no publisher certificate or qualified Windows release host is available here. R2 Linux host/recovery and remote-workbench qualifications, Windows clean-VM/package checks, and independent R3 content/research quality, cost, recovery and organization-benefit evidence remain separate open work.

Slice216 was the planned next source stage when this snapshot was written. It is superseded by Slices267–268 and the final Slice268 source audit. The current local status, open qualification gates, and re-audit conditions are recorded in `docs/implementation/NEXT_SLICE.md` and `docs/implementation/OPEN_QUALIFICATION_BLOCKERS.md`.

## Working conventions

- Follow `AGENTS.md`: Go/PostgreSQL 18/pgx/sqlc/goose, migrations only forward, evidence under `evidence/development/`, and RTK-prefixed shell commands.
- Preserve exact provider-surface fingerprints and old probe registries. A new model-visible tool set needs a distinct versioned surface and independent qualification.
- Use disposable PostgreSQL/file roots for qualification.
- Keep verification scoped to the user's request; builds and `git diff --check` do not qualify frozen scenarios. External qualification remains gated by its required host/account and separate authorization.
- After every completed stage, commit and push to GitHub `main`, then fetch and verify the remote file tree before continuing. This is source synchronization only; it does not authorize deployment or external provider actions.
