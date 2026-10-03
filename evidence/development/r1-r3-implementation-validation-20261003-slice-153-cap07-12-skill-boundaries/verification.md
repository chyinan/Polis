# Slice 153 — CAP-07–12 Skill discovery, loading and revocation audit

Date: 2026-10-03

## Scope

Read-only source audit of Skill package import, version binding, Worker tool exposure and revocation. No code/schema change, local directory scan, script/service execution, network access, test suite or database-backed WorkerSession run was performed.

## Findings

- **CAP-07:** `TXImportReadOnlySkillPackage` accepts an explicit uploaded ZIP. `PrepareReadOnlySkillBundle` canonicalizes a bounded package and accepts only `SKILL.md`, Markdown/text references, supported static assets, README and license files. Hidden paths and other paths/extensions (including hooks and scripts) are rejected. Import records a candidate; it does not inspect host Skill/config directories or start/model-install/network anything.
- **CAP-08:** storage keys revisions by Company, publisher scope, package ID and revision. Supported scopes are `group` and `company`; Project scope is absent. Worker loading and binding use exact immutable Skill IDs and content digests rather than same-name path lookup, so a different same-name revision cannot silently replace an existing grant.
- **CAP-09:** `skills_load` requests one exact relative path, verifies all manifest CAS entries and returns only the selected bounded UTF-8 text file. Every load records Skill ID, package, revision, version digest and exact path; repeated activation is idempotently recorded. However, current Handover can include 64 Skills and up to 250 file metadata records per Skill, with no aggregate reference cap.
- **CAP-10:** the parser accepts bounded `allowed-tools` metadata but strips it from the canonical permission projection. Worker tools remain a fixed registry; loaded content is marked `approved_static_text_no_additional_permissions`, and tool argument parsing remains strict.
- **CAP-11:** there is no Skill-script or Skill-dependency executor. Executable and unsupported package paths are rejected, so script execution fails closed. This does not satisfy the approved-script sandbox execution scenario; no install or command fallback exists in the Skill path.
- **CAP-12:** revisions are immutable and each Worker binding pins the exact content digest. Grant changes, Skill loads, and revocations serialize on the Company lock. Reconnect replay of `skills_load` rechecks the current grant before returning content. Revocation snapshots affected WorkerSessions and the durable stopper stops/reconciles exact sessions; session executions were not run here.

## Proposed bounded directory continuation

Add a separately versioned fake-only Worker surface with a keyset-paged `skills_list` taking an exact bound Skill ID. It returns a fixed-size path metadata page and continuation key. That surface's `work_current` response should carry bounded Skill summaries without expanding every package's full path list. Every list/load request must be checked against the current database-confirmed active WorkerSession and its exact employee binding. Preserve the existing @5 tool registry and response behavior; do not add script execution or host-path access.

## Verification

- Source references reviewed: `internal/capabilitysource/skill_bundle.go`, `internal/kernel/capability_skill_source.go`, `internal/kernel/capability_skill_runtime.go`, `internal/kernel/capability_governance.go`, `internal/kernel/capability_revocation_stop.go`, `internal/codex/transport_turn.go`, `internal/control/capability_revocation_stop.go`, and `spec/design-v0.4.5/ARCHITECTURE.md` CAP-07–12.
- No code or schema diff was produced; the local dev database remains unchanged at Schema 98.
- CAP-07–12 scenarios, live WorkerSession E2E, concurrent grant/revoke qualification, host directory scanning and script sandbox qualification remain `not_run`.

## Files

- `docs/implementation/NEXT_SLICE.md`
- `docs/implementation/PROGRESS.md`
- `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`
