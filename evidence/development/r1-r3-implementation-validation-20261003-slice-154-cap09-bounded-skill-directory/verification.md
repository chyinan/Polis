# Slice 154 — CAP-09 bounded Skill file discovery

## Scope

Added a new Fake-only product Worker surface, `polis-product-tool-surface@9`, with `skills_list`. A request names one exact Skill ID and a path cursor; the response contains at most 32 file metadata records and a continuation cursor. The @9 `work_current` response omits Skill file-reference arrays so the Worker can discover files through bounded pages. Existing @5 tools, manifest and behavior are unchanged.

The Kernel listing path holds the Company row lock while it checks the active database WorkerSession, current owned working Task, exact employee Skill binding, approved revision and complete CAS bundle. It discloses only canonical path metadata for the immutable verified package. Skills remain static guidance; the surface adds no script execution or host path access. `POLIS_OFFLINE_SKILL_DIRECTORY_ENABLED=1` is opt-in and accepted only with `POLIS_WORKER_MODE=real` and `POLIS_PROVIDER_TRANSPORT=fake`. Real-provider authorization remains unchanged.

## Pinned surface

- Qualification: `polis-product-tool-surface@9`
- Tool count: 9
- Manifest digest: `7ad11ab318f15e124554709b8b5bfed2e87f60d4fcad07428def7e41fd1390e1`
- Aggregate schema: 2724 bytes, SHA-256 `8ffefe6b5f3399dfb480ab9d77e063b4e407074d45c95a0cf4e85a0603ad788e`
- Page size: 32 file records
- Schema migration: none (Schema 98 remains unchanged)

## Verification

- `gofmt` on changed Go files: passed
- `go build ./cmd/polis ./internal/kernel ./internal/control ./internal/provider ./internal/codex`: passed
- `git diff --check`: passed
- Tests: not run
- Live PostgreSQL-backed WorkerSession E2E: not run
- Frozen CAP scenario execution: not run; all remain `not_run`

This verifies compilation and the pinned tool-surface metadata, not a live WorkerSession operation or CAP qualification. The active WorkerSession database gate is implemented in the Kernel path; runtime E2E evidence remains outstanding.
