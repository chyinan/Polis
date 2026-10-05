# Open R1–R3 qualification blockers

Updated: 2026-10-05, Slice 247. This is an execution handoff for the still-open requirement IDs in `R1_R3_TRACEABILITY_DISPOSITION.json`; it does not close, downgrade, or re-scope any requirement or frozen scenario.

## Current checkout evidence

- Audit base for Slice247: published Slice246 implementation commit 97db192c9659582e2e5e72e05da21f0617fb3b6e. Slices242–247 changed source/handoff records only; the host and database observations below were not refreshed.
- Most recent recorded host observation: Android/Termux (`uname` reported Android; Node platform was `android`); `pwsh`, `powershell`, `bwrap`, and `systemd-run` were absent in the Slice235 audit. This is a recorded observation, not a fresh Slice241 host probe.
- Most recent recorded database observation: a read-only query during Slice239 found Schema 108, 0 Companies, 0 Missions, and 0 active WorkerSessions. Slices240–247 did not refresh the database observation; this remains the latest recorded evidence, not a fresh query.
- The checked-in traceability ledger lists 17 open software requirements and 232 frozen scenario rows; the current JSON parse/count confirms every scenario has `executionStatus: not_run`.

These facts rule out session-backed execution in the current database and native Windows or delegated Linux qualification here. They do not prove that external accounts, certificates, or hosts do not exist elsewhere; none has been provided or connected for these product qualification tasks in this checkout.

## Requirement-by-requirement prerequisites

| Open requirement | Remaining gate in the approved ledger | Evidence needed to resume |
|---|---|---|
| REQ-02 | Company creation now accepts only the four canonical fixed ID/role pairs; the wizard presents those roles read-only and includes them in final review. Existing-company acknowledgment checks the persisted mapping atomically, and company updates cannot change roles before or after acknowledgment. Acknowledgment projections reject stale digests or noncanonical persisted mappings; a strict semantic validator also rejects changes to the embedded draft flags, canonical task assignments, checker policy, or qualification state. Roles remain unverified and execution disabled. Provider role contracts, executable admission gates, and direct messaging remain unqualified; no owner confirmation has been made in this checkout. | Installation owner reviews and confirms the fixed roster/role mapping in the UI; separately authorized real-provider account and role-surface qualification. Legacy persisted role mismatches require owner-reviewed reconciliation before acknowledgment. |
| REQ-13 | The Company-ID cursor now matches candidate ordering, preserving cross-Company fairness across wraparound; the Fake-only dispatcher retains per-Task claims. No owner-backed global slot cap or authoritative quota-ready/release source exists. | Owner decision for the installation-wide cap and a trusted provider quota signal/account source. |
| REQ-14 | Legacy revocation rows may still need an installation-owner `acknowledged_unresolved` disposition; stop/restart proof depends on runtime host state. | Owner review of each applicable legacy row and host-backed exact stop/recovery evidence. |
| REQ-15 | Memory overlay writes now recheck Company-directory permissions at runtime; restore behavior still needs PostgreSQL/Desktop qualification, and corrections/revalidation still require a session-bound operator path. | A qualified Desktop/PostgreSQL restore host and an already-active, database-confirmed WorkerSession for any session-bound action. |
| REQ-16 | ProviderAccount identity/liability, hidden retries, and token/money accounting lack a trusted billing scope. | Owner-confirmed billing mode and liable account identity, provider accounting evidence, and exact retry/charge qualification. |
| REQ-23 | Durable closeout code is present; restart/recovery and FT-57–60/72 qualification remain open. | Runtime restart/recovery host with representative persisted work and separately authorized frozen-scenario execution. |
| REQ-24 | The formal employee operation path has not been qualified against an active real or approved session. | An already-active WorkerSession confirmed by a current database query and its qualified runtime/provider surface. |
| REQ-25 | Owner setup is implemented, and logout/authorization invalidation now clears views across same-origin tabs; local browser/Tauri WebView and remote-browser authorization behavior remain unqualified. | Installation-owner setup decision plus supported browser/WebView qualification environments and explicit remote-access authorization. |
| REQ-26 | No concrete shared branch/deploy/publish writer exists to fence; an inert ResourceKey registry would not enforce isolation. | A product-approved shared write action and its canonical provider object/account mapping before implementing a binding gate. |
| REQ-27 | Product-provider successor admission now requires the prior exact profile; cross-profile transitions remain denied without an owner-approved contract. E-HANDOVER regression and owner-approved budget/threshold evidence remain open. | Authorized model/provider accounts, an active session path, and owner-approved continuity thresholds. |
| REQ-29 | The logical private Task tree and snapshot paths exist; the legacy `workspace_replace` writer now shares the 512-file, 16 MiB, and path-prefix invariants. Host mounts, company/group shared roots, retention/GC decisions, and CAP-01–06 qualification remain open. | Owner-approved sharing and retention policy, qualified Windows/Linux filesystem hosts, and CAP evidence for each exposed path. |
| REQ-30 | Read-only Skill loading is implemented on Fake; real provider v5 remains unqualified. | Authorized real-provider account and active WorkerSession on the exact v5 surface. |
| REQ-31 | Real-provider capability execution, Windows WFP, detached-owner recovery, and R2 Streamable HTTP endpoint/Worker qualification remain open. | Qualified Windows host for WFP, authorized MCP endpoints/accounts, and active database-confirmed WorkerSession for session execution. |
| REQ-32 | Identity bindings are persisted; product-provider successor admission also requires profile continuity, while successor execution remains unavailable. | Qualified successor WorkerSession/runtime and explicit owner authorization to exercise bound capabilities after handover. |
| REQ-33 | Truthful capability metadata is implemented; live execution remains unavailable pending runtime qualification. | Qualified host/provider for the exact capability surface plus owner-approved qualification evidence. |
| REQ-34 | Governance records and dispatch fences exist; real-provider execution remains unavailable. | Qualified provider/host, owner authorization, and active-session evidence for each exposed dispatch surface. |
| REQ-39 | Planning assessment and owner review fences are implemented; session-backed and frozen-scenario qualification remain open. | An already-active `emp-planning` WorkerSession confirmed in the database, plus authorized execution of the eight mapped REQ-39 scenarios. |

## Shared host and authorization gates

- **Windows:** native AppContainer, WFP/registry, Node/npm, packaged recovery, browser, and clean-VM qualification require a Windows qualification host. Authenticode verification additionally requires a valid publisher certificate and timestamp path.
- **Linux:** delegated Worker containment and environment qualification require a Linux host with the delegated cgroup v2 root, the approved isolated workspace volume, and restart/multi-day evidence. This Termux Android host does not provide the required `bwrap` toolchain.
- **Provider/account actions:** GitHub repository-write access used to push this source checkout is not evidence for the product's GitHub feedback, model-provider, QQ, MCP, or billing integrations. Those require their own credentials, target scope, and authorization.
- **Worker actions:** do not create or start a Worker to satisfy the qualification prerequisites. Proceed only after a current read confirms an already-active WorkerSession; the present database reports zero.
- **Frozen scenarios and R3 evidence:** all scenario execution remains `not_run`. Scenario runs, real domain outcomes, and organization-benefit evidence require their own authorization, data, and qualified environment.

The source implementation can continue where a concrete local gap remains. Do not infer completion from these prerequisites being documented; keep the requirement and scenario dispositions open until their evidence exists.

## Repository push status

The ordinary local HTTPS `git push` fails because no command-line GitHub credential is configured. The connected GitHub integration confirmed push permission and published the implementation commits for Slices 236 (`56b69a51ea93b7226056bcb508912453365b9690`), 237 (`576ebb7476da17e1e6bcdfdb2dd57443c601beab`), 238 (`7406d45aac38b7bf380ecb4e82db2ac137940feb`), 239 (`6482bad78cc14b590b8d6c36be5939a141588591`), and 240 (`b592aa4f839058e53663dac8315a96bb32c0ff2d`) to `main`; the handoff-pointer reconciliation commits are also in the linear history. Slice240 started from local and remote `main` at `ccafcc11d3d5cc2f63b8d0f7faa0ee02db93d001`. No token was copied into local files or environment variables. This repository write credential does not qualify any product provider integration.
