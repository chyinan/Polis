# Remaining product feature implementation plan

Status: active. Increment 1 (REQ-37 Task-bound project JobRun tools) is implemented and offline-verified in `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-req37-project-jobs-20261026/verification.md`. REQ-36 increments 2a–2c (Worker ensure bridge, pure overlay validation, and durable owner materialization flow) are source-implemented; see the Slice325/326/327 verification records. Slices328–329 add the BrowserRun/ResearchOperation execution authorization fence and isolated fake @18/@19 surface wiring; concrete adapters remain blocked because no qualified authorizer/credential resolver is installed. Slice330 adds owner-reviewed fixed-role drift repair; Slice331 exposes the existing TaskRevision owner decision lifecycle without enabling execution. Slice332 marks page rendering unsupported in new PDF records. Slice333 selects conservative REQ-29 defaults: Company-private CAS snapshots, no host-root authority from `workspace_root`, no host writes, no cross-Company sharing, and no automatic snapshot GC. Slice334 adds a bounded `os.Root` snapshot primitive, not connected to a caller. Slice335 adds local review/explicit confirmation to browser-selected directory imports; this does not authorize server access to `workspace_root`. Slice336 hardens semantic TaskRevision mapping against mutation or stale/revoked RoleRevision state without changing execution policy. Slice337 reviewed renderer candidates without selecting a dependency. Slice338 confirms the pinned parser does not expose a page paint stream; full-page rendering remains unsupported pending a parser-owned bounded drawing IR or a verified renderer. Native root binding remains unavailable pending durable owner authorization and host qualification. No real registry/npm/retrieval operation was run.

## Goal and boundaries

Close the source-level product gaps identified in `OPEN_QUALIFICATION_BLOCKERS.md` and the current REQ ledger. A feature is not considered complete merely because its control-plane row or fake-only interface exists: the intended product path must be wired, bounded, and testable. Host/provider/account qualification and frozen-scenario execution remain separate gates and must not be represented as passed by offline implementation work.

This plan is limited to product code and disposable offline verification. It does not authorize real Provider requests, external retrieval, QQ/MCP/GitHub business operations, production database writes, or creation/start of a real Worker. Any new execution capability remains opt-in, versioned, and default-deny when its exact adapter, policy, or host qualification is absent.

## Increment order

### 1. REQ-37 — Task-bound Worker project JobRun start/stop

- Add a new exact fake-only Worker surface; preserve historical `@4`, `@15`, and `@17` fingerprints.
- Reuse `Control.StartProjectJob` / `StopProjectJob`, rather than adding a second executor or process manager.
- Derive Company, Task, and WorkerSession only from the trusted binding. Accept only the existing bounded `batch` or registered `service` request forms; validate the selected environment and current executor through existing Control gates.
- Stop only a JobRun owned by the current Task and exact WorkerSession. A missing Control bridge, inactive session, stale scope, unprepared environment, or unqualified executor must fail closed.
- Keep the extension disabled unless `POLIS_OFFLINE_PROJECT_JOBS_ENABLED=1` and `POLIS_WORKER_MODE=real` with `POLIS_PROVIDER_TRANSPORT=fake`; do not authorize it on the real Provider `@4` surface.
- Verify schemas/fingerprints, default-deny and scope guards, then disposable PostgreSQL-backed start/stop lifecycle with an injected recording executor. Test-only sleeper process used to establish a Kernel-confirmed WorkerSession; no native ProjectJob process, real Worker turn or Provider request ran.

### 2. REQ-36 — Approved dependency materialization

- Completed Slice 325: the Worker `environment_ensure` request now reaches the existing Control preparation executor only through the active WorkerSession/Task/Mission guard and the exact approved environment revision. The bridge remains fake-only @16 and default-off.
- Completed Slice326: pure functions apply only exact approved versions to root `dependencies`, preserve other manifest values, and validate the generated lockfile against the exact overlay, registry subset, tarball host/integrity rules, and profile bounds. Duplicate/case-variant JSON fields are rejected. Evidence: `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-req36-overlay-validator-20261026/verification.md`.
- Completed Slice327: owner Workbench API/UI now lists, proposes, approves/rejects and explicitly materializes DependencyChanges. Schema135/136 persists immutable attempt identities/events, enforces one-shot request IDs and records output references/log digests; startup converts stale `running` attempts to `outcome_unknown`, never replaying a resolver. Control validates exact outputs, stores a derived immutable directory MissionInput and registers a successor environment revision; the successor retains the policy but is not approved automatically. The Windows executor now has a fixed lockfile-only npm command inside the registry-only AppContainer, limited to one proposal registry and `--ignore-scripts`; Linux remains unsupported under its deny-all policy. Evidence: `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-req36-materialization-control-20261026/verification.md`.
- Remaining REQ-36 qualification: run no resolver until the Windows bounded-volume/AppContainer/WFP/cgroup-equivalent host boundary is actually qualified and the owner explicitly invokes the approved operation. No native npm/registry execution was performed in Slice327.
- Implement only the preselected registry allowlist, no-lifecycle-script, bounded timeout/output, and fixed network envelope; do not let Worker input choose commands, host paths, or executor profiles.
- Keep approval distinct from materialization. Missing host qualification or executor keeps the request denied/blocked.
- Verify exact replay, stale revision, cancellation/recovery, and fail-closed executor behavior using disposable data and injected fakes.

### 3. REQ-38 — BrowserRun/research product adapter wiring

- Completed Slice328: Kernel now requires a separate explicit execution authorizer in addition to adapter injection; absent/negative authorization leaves BrowserRun `blocked` and ResearchOperation `unavailable` without calling the adapter. Existing Playwright and research adapters do not implement a qualified authorizer and therefore stay fail-closed.
- Completed Slice329: exact opt-in fake-only @18/@19 surfaces are selectable on the Fake transport, while adapter enablement requires one such isolated surface. The authorization fence still blocks external execution; no endpoint, credential or host qualification was inferred.
- Wire only exact configured origins, credentials by protected reference, result bounds, and evidence Artifact checks. Never add ambient cookies, arbitrary headers, redirects, or caller-selected upstream URLs.
- Keep execution unavailable until provider credential resolution, pinned endpoint/ranking, isolated profile/network policy, and qualified runner are all present.
- Verify configuration, deny paths, evidence binding, and no-network default with offline tests; do not perform external retrieval.

### 4. REQ-02 — Executable Task/Role contracts

- Completed Slice330: historical fixed-role drift is projected to the owner and can be restored only to the embedded canonical mapping after exact-snapshot review, with no active work and a concurrent-admission fence. Qualification remains `unverified` and execution remains disabled.
- Completed Slice331: the Schema123 TaskRevision owner-decision command is reachable at a path-bound task route and visible in Task detail; approval/rejection/revocation remain descriptive and do not qualify or enable execution.
- Completed Slice336 regression guard: stale, revoked, missing, or mutated compiled RoleRevision values cannot produce TaskRevision coverage; they resolve as unavailable/unverified and remain human-gated. Existing owner approval does not grant qualification or execution.
- Typed fixed-team TaskKind/RoleRevision binding, TaskRevision persistence/projection and owner-decision audit are implemented. Remaining REQ-02 work is qualification and separately approved admission policy; do not infer execution authority from approval.
- Do not infer executable semantic mappings from display labels or fixed-roster confirmation alone.
- Slice336 adds default-deny regressions for missing/stale/revoked/mutated RoleRevision values; the existing TaskRevision lifecycle test rejects resurrection after revocation.

### 5. REQ-35 — Full-page PDF representation

- Completed Slice332 fallback contract: new `polis-pdf-extraction@4` records explicitly state `fullPageRenderingStatus=unsupported` and a reason/warning when no bounded renderer is present. Historical `@1`/`@2`/`@3` packages remain canonical and verifiable. This is a truthful fail-closed boundary, not full-page rendering.
- Completed Slice337 candidate review: neither AGPL-3.0 `go-fitz` nor the embedded `go-pdfium` WASM artifact was selected. The exact WASM third-party license inventory and lower, tested per-render resource envelope remain unproven; `unsupported` remains the safe product contract.
- Completed Slice338 feasibility review: the existing parser exposes glyph/image extraction, not a public painting-operator stream. An in-house complete renderer would require a parser extension/fork and complete operator coverage; composing the current outputs would violate the truthful full-page contract.
- Design a bounded deterministic page-rendering contract compatible with canonical PDF snapshots, preserving source bytes and existing extraction versions.
- Select an implementation only after checking dependency/license, memory/pixel/output caps, and fidelity/omission semantics. If the renderer cannot be included safely in the existing runtime envelope, leave page rendering explicitly unsupported rather than silently approximating it.
- Test malformed, oversized, masked, multi-page, and non-rasterizable cases offline; provider image qualification stays separate.

### 6. REQ-29 — Host-root, sharing, and retention

- Completed Slice333 policy choice: Company-private access only; `workspace_root` remains metadata, not authority; imports are explicit read-only snapshots in Company CAS; no Worker host writes, cross-Company/Group sharing, or automatic reclamation. Revocation blocks later reads but keeps CAS references.
- Completed Slice334 foundation: `PrepareReadOnlyRootSnapshot` accepts an already-open `os.Root`, emits only bounded relative CAS snapshot data, and rejects observed symlinks/special files. It is not an owner binding, is not wired to Workbench/Worker, and does not claim host race qualification.
- Completed Slice335 UI safety increment: browser-selected directories show their bounded file list and likely-sensitive path hints, then require explicit confirmation before upload. The server root metadata is never read by this route; native root binding remains a separate blocked feature.
- The selected future root-binding candidate is read-only and Company-scoped with no cross-Company sharing, no-follow traversal, bounded snapshots, and retained immutable CAS references. It remains unavailable until an OS-native owner-selected root handle, no-follow/special-file/TOCTOU implementation and host qualification exist.
- Cover durable native authorization, symlink/special-file/TOCTOU, revocation, and retention counters before exposing any native-Company-root UI or route; the existing browser file-selection UI remains a separate user-granted CAS upload.

### 7. REQ-13 / generic actions / R3 domain operations

- Integrate provider-authoritative quota readiness only through a typed, authenticated provider signal; until then do not release `waiting_quota` automatically.
- Keep generic ActionIntent/ResourceKey writes deny-by-default until a concrete shared writer, object/account mapping, idempotency, and recovery contract exist.
- Treat R3 research/content operation quality, intervention, cost, correction, and organization-benefit evidence as a separate product/qualification workstream; do not equate deterministic simulator coverage with real-domain effectiveness.

## Completion rule

Each increment gets a dedicated evidence record and a commit/push to the current feature branch after the focused and regression checks pass. The top-level task is not declared fully complete until every source-level item is implemented or explicitly retained as a named product-policy/host/provider gate, and the qualification ledger clearly distinguishes those states.
