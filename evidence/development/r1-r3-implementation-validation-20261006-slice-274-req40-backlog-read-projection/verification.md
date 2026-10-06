# Slice 274 — REQ-40 Company backlog read projection

Date: 2026-10-06

## Scope

This offline slice adds a read-only projection for Schema 113 terminal delivery feedback backlog events. It is scoped to the current Company and Artifact delivery, capped at 32 `open` events, validated at the Go read-store boundary and the frontend DTO boundary, and displayed in the existing durable delivery panel. A pre-Schema114 runtime without the optional table returns an empty backlog; Schema114 adds the delivery-leading index. It does not create Tasks, wake Missions, modify terminal state, send notifications, or invoke external services.

## Verification

- `frontend/node_modules/.bin/vitest.cmd run src/domain/workbench-validation.test.ts -t "durable delivery backlog projection"` — PASS (1 targeted test; remaining tests skipped).
- `frontend/node_modules/.bin/vitest.cmd run src/data/real-workbench-api.test.ts -t "legacy durable delivery response"` — PASS; old responses without `feedbackBacklog` normalize to an empty array.
- `pnpm --dir frontend build` with a temporary worktree-only `allowBuilds.esbuild=true` override, restored afterward — PASS; existing large-chunk advisory only.
- direct `tsc -b` and `vite build` after the compatibility fix — PASS; existing large-chunk advisory only.
- `rtk bash scripts/go.sh test ./internal/workbench -run TestDoesNotExist -count=1` — PASS, compile-only.
- `rtk bash scripts/go.sh test ./db -count=1` — PASS, migration/hash coverage through Schema 114.
- `rtk bash scripts/go.sh build ./cmd/...` — PASS.
- `rtk git diff --check` — PASS.

The broader frontend test file still has three pre-existing failures unrelated to this projection: formal Mission-change fixture validation and mission tool-call-limit fixture validation. No PostgreSQL runtime, Worker/provider, browser, external account or frozen scenario ran; SQL row decoding, fallback and index behavior remain unexecuted integration coverage.
