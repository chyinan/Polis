# R1 bound read-only Skill loading foundation

Date: 2026-09-25; continuation: 2026-09-27

## Behavior

The Worker-facing `skills_load` operation loads one exact text file from a Skill revision that is currently approved, source-verified, and bound to the active employee. The request supplies the Skill ID and exact package-relative path. The Kernel checks the active WorkerSession, company, latest employee binding event, pinned content digest, qualification record, and latest approval decision before returning any bytes.

The Kernel reads every package member back from company CAS, verifies the read-only package manifest and every member digest, then returns only UTF-8 Markdown/plain text up to 64 KiB per load. The response includes the Skill revision, version digest, selected path, media type, content digest, and a boundary label stating that the text grants no additional permissions. Images and other non-text files appear in the catalog as unavailable for this operation; no scripts, dependencies, or MCP calls are executed.

`work_current` exposes only Skills bound to that employee, together with exact revisions, package digests, and explicit reference paths. Repeated loads of the same reference in a WorkerSession reuse one load reference. Each first load appends a company event containing the Employee, Task, session, revision and file digests, but not the Skill body. The handover projection carries this catalog and the Task's recent load references so a successor can see what was used and reload it only if their own binding permits.

The loader rechecks authorization for every new call and every idempotent replay. The native transport also redispatches a replayed `skills_load` request to the Kernel instead of returning cached Skill text, so revocation is checked again after reconnect. Employee unbinding, Skill revocation, Task/session stop, or version mismatch blocks further content return. Load history remains visible after revocation.

## Provider surface boundary

The qualified `polis-product-tool-surface@4` remains byte-for-byte unchanged, including its `work_current` result shape. The new `ProductSkillToolSurface()` is a separate eight-tool `polis-product-tool-surface@5` surface; only v5 `work_current` includes the bound Skill catalog and recent load references. The control adapter dispatches `skills_load` only when that exact surface and qualification ID are selected. The current provider authorization gate accepts only the qualified v4 identity, so v5 cannot start a real model turn. This slice implements the source, authorization, audit, handover and dispatch code without claiming provider qualification.

The deterministic fake runtime now has an explicit `ReadOnlySkillSurface` mode. It accepts only the pinned v5 manifest `f1fe445f46675e7d1883c3b9360f0ad6bef168ce2f5a2ebc8ea97e9a1f28b2a8` and aggregate schema `a43fb0039ccc3e2b654fdf2549ac22432f690f95eb16157ee8373e1e6d059ed0` (2,459 bytes), with an `offline-skill-surface-unqualified` marker. It selects the first catalog-listed loadable `SKILL.md`, calls the same `skills_load` Kernel path, and requires a bounded static-text response. `RealProviderWorkerAdapter` permits this profile only when the underlying runtime reports `fake`; the real-provider gate still accepts only the qualified v4 surface. The zero-egress PostgreSQL integration records the Skill ID, exact version digest and path without persisting the Skill body. This exercises Worker-to-Kernel dispatch without making a provider qualification claim.

## Limits

- Only Markdown/plain-text files no larger than 64 KiB can be loaded. PNG/JPEG Skill assets are imported and listed, but this provider tool does not attach them as typed image content.
- Skill text is approved static guidance. It cannot grant tools, override Polis authorization or the Task contract, execute scripts, install dependencies, or open network access.
- Exact provider surface v5 qualification, live model Skill-context behavior, cross-backend behavior, and real model handover remain gated and unqualified. The fake runtime is a deterministic control-path fixture and is not evidence that a model applied Skill guidance.

## Verification

The original Kernel/runtime foundation evidence is recorded in `evidence/development/r1-r3-implementation-validation-20260925-slice-35/verification.md`. The offline fake Worker integration is recorded in `evidence/development/r1-r3-implementation-validation-20260927-slice-47-offline-skill-worker/verification.md`.
