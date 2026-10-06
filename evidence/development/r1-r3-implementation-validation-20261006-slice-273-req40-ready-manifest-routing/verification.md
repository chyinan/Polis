# Slice 273 — REQ-40 ready Manifest and lifecycle routing

Date: 2026-10-06

## Scope

This source-only slice continues the Slice 272 handoff. It adds:

- a pure canonical ready-Manifest builder with five bounded evidence sections;
- an owner-authenticated/CSRF-protected completion command that appends a ready revision from the current assembling revision;
- source-input, validation-binding/runner and validation-contract digest binding;
- active/paused formal change-request routing for `changes_requested`;
- an immutable Schema 113 Company backlog event for terminal Missions, guarded against accepted dispositions and non-terminal Missions.

No user acceptance is inferred. The new ready revision starts with `not_requested`; downloads, previews, notifications and internal validation remain separate.

## Verification run

All commands ran in the clean `D:\Programs\Polis-cloud-main` worktree on branch `codex/req40-manifest-routing`.

- `rtk bash scripts/go.sh test ./internal/kernel -run TestBuildReadyProductDeliveryManifest -count=1` — PASS.
- `rtk bash scripts/go.sh test ./internal/kernel -run TestValidateProductDeliveryManifestCompletionCommand -count=1` — PASS.
- `rtk bash scripts/go.sh test ./internal/kernel -run TestCompleteProductDeliveryManifestRejectsMalformedCommandBeforeDatabaseAccess -count=1` — PASS.
- `rtk bash scripts/go.sh test ./internal/kernel -run TestProductDeliveryChangeRouting -count=1` — PASS.
- `rtk bash scripts/go.sh test ./internal/kernel -run TestProductDeliveryDispositionRouteLeavesAcceptedAlone -count=1` — PASS.
- `rtk bash scripts/go.sh test ./internal/workbench -run TestCanonicalDurableDeliveryManifestDoesNotHTMLEscapeEvidence -count=1` — PASS.
- `rtk bash scripts/go.sh test ./internal/kernel -run TestDoesNotExist -count=1` — PASS, compile-only.
- `rtk bash scripts/go.sh test ./internal/control -run TestDoesNotExist -count=1` — PASS, compile-only.
- `rtk bash scripts/go.sh test ./internal/workbench -run TestDoesNotExist -count=1` — PASS, compile-only.
- `rtk bash scripts/go.sh test ./db -count=1` — PASS, including migration hash coverage for Schemas 112–113.
- `rtk git diff --check` — PASS.
- `pnpm --dir frontend build` with a temporary worktree-only `allowBuilds.esbuild=true` override, restored afterward — PASS; Vite emitted only the existing large-chunk advisory.

## Baseline and limits

The pre-change full `rtk bash scripts/go.sh test ./...` was also run after installing the pinned Go 1.26.8 toolchain. It retained pre-existing failures in `internal/control` (9 tests), `internal/kernel` (1 test), and `internal/provider` (1 test), while the remaining packages passed. Those failures are unrelated to the REQ-40 files changed here and were not altered.

No dedicated PostgreSQL DSN was configured, so completion/backlog transactions were not executed against a database. No WorkerSession, provider turn, MCP endpoint, browser, GitHub, QQ, external account or frozen scenario was started. Recorded runtime remains Schema 108; source migrations are Schema 113; all 232 frozen scenarios remain `not_run`.
