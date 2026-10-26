# Remaining product feature implementation plan

Status: active; increment 1 (REQ-37 Task-bound project JobRun tools) is implemented and offline-verified in `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-req37-project-jobs-20261026/verification.md`.

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

- Trace the approved DependencyChange through existing environment revision and preparation services.
- Implement only the preselected registry allowlist, no-lifecycle-script, bounded timeout/output, and fixed network envelope; do not let Worker input choose commands, host paths, or executor profiles.
- Keep approval distinct from materialization. Missing host qualification or executor keeps the request denied/blocked.
- Verify exact replay, stale revision, cancellation/recovery, and fail-closed executor behavior using disposable data and injected fakes.

### 3. REQ-38 — BrowserRun/research product adapter wiring

- Reconcile the existing default-off BrowserRun and ResearchOperation adapters against current schemas and owner configuration.
- Wire only exact configured origins, credentials by protected reference, result bounds, and evidence Artifact checks. Never add ambient cookies, arbitrary headers, redirects, or caller-selected upstream URLs.
- Keep execution unavailable until provider credential resolution, pinned endpoint/ranking, isolated profile/network policy, and qualified runner are all present.
- Verify configuration, deny paths, evidence binding, and no-network default with offline tests; do not perform external retrieval.

### 4. REQ-02 — Executable Task/Role contracts

- Complete the typed fixed-team TaskKind/RoleRevision binding and approval/audit projections while retaining `unverified` and execution-disabled until explicit owner review and qualifications exist.
- Do not infer executable semantic mappings from display labels or fixed-roster confirmation alone.
- Add default-deny regression tests for missing/stale/revoked role and TaskRevision bindings.

### 5. REQ-35 — Full-page PDF representation

- Design a bounded deterministic page-rendering contract compatible with canonical PDF snapshots, preserving source bytes and existing extraction versions.
- Select an implementation only after checking dependency/license, memory/pixel/output caps, and fidelity/omission semantics. If the renderer cannot be included safely in the existing runtime envelope, leave page rendering explicitly unsupported rather than silently approximating it.
- Test malformed, oversized, masked, multi-page, and non-rasterizable cases offline; provider image qualification stays separate.

### 6. REQ-29 — Host-root, sharing, and retention

- Add no shared-root or reclamation behavior until the access owner, Company/Group sharing model, revocation semantics, and retention policy are explicitly encoded in a fail-closed contract.
- First candidate is a read-only, Company-scoped native root binding with no cross-Company sharing, no-follow traversal, bounded snapshots, and retained immutable CAS references; it remains unavailable until the platform adapter is qualified.
- Cover authorization, symlink/special-file/TOCTOU, revocation, and retention counters in tests before exposing a UI path.

### 7. REQ-13 / generic actions / R3 domain operations

- Integrate provider-authoritative quota readiness only through a typed, authenticated provider signal; until then do not release `waiting_quota` automatically.
- Keep generic ActionIntent/ResourceKey writes deny-by-default until a concrete shared writer, object/account mapping, idempotency, and recovery contract exist.
- Treat R3 research/content operation quality, intervention, cost, correction, and organization-benefit evidence as a separate product/qualification workstream; do not equate deterministic simulator coverage with real-domain effectiveness.

## Completion rule

Each increment gets a dedicated evidence record and a commit/push to the current feature branch after the focused and regression checks pass. The top-level task is not declared fully complete until every source-level item is implemented or explicitly retained as a named product-policy/host/provider gate, and the qualification ledger clearly distinguishes those states.
