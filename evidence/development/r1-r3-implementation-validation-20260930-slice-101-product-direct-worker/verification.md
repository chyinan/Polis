# Slice 101: Product Worker direct messaging

Date: 2026-10-01 (Slice101 began 2026-09-30)

Base commit: `d2ce452c1c139ab409df7b9885efff79228e41bf` (Slice 100)

## Scope

Connected the existing durable message and Obligation lifecycle to the generic product `EmployeeTools` adapter through a separately versioned 12-tool `polis-product-tool-surface@7` extension. The extension includes exact same-Mission target discovery, direct send, inbox, acknowledgement, workspace-application evidence, and candidate-Artifact resolution. Each send locks the Mission row `FOR SHARE`, then locks source and target Task rows in ID order before checking active Mission and Task state. The frozen `@4`, Skill `@5`, guidance `@6`, and historical peer tool registries remain unchanged.

The direct-message surface is accepted only by the explicit zero-egress Fake Runtime profile. Its exact manifest is `82d7b2dbc41ff3dbed56813b3b3adcfad48818fb29653bcd2debf1f280e507eb`; aggregate schema is 3503 bytes with SHA-256 `769f7c9f4afb1c1d0ebfb43037a06d8f2ddb661bc111970c4d199f48f962fd42`. The real-provider authorization path remains pinned to @4 and rejects @7.

No database migration was added. `core.TaskKind` still admits only `compat/emp-backend` to the product-provider Task path. The direct-message tools and adapter wiring do not qualify other fixed roles or a real provider surface.

## Verification

- `rtk bash scripts/go.sh test ./internal/codex ./internal/kernel ./internal/provider ./internal/control` — passed.
- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` — passed on a dedicated disposable PostgreSQL 18 instance through Schema 72. Covers actionable send/inbox/ack/apply/check/candidate/resolve, FYI ordering and no obligation/schedule wake, explicit target list, cross-Mission refusal, missing-actionable rejection, concurrent send/submission ordering, concurrent Mission-pause ordering, and existing peer regressions.
- `rtk bash scripts/go.sh test ./...` — passed.
- `rtk bash scripts/go.sh build ./cmd/...` — passed for Linux amd64.
- `rtk bash -lc 'GOOS=windows GOARCH=amd64 ./scripts/go.sh build ./cmd/...'` — passed for Windows amd64.
- `rtk git diff --check` — passed after source, evidence, and handoff updates before the closeout commit.

Provider tests assert that exact @7 is accepted only under the Fake Runtime purpose/envelope/markers, while real mode, altered surface identity, and mixed Skill/direct profiles are denied. Review follow-up added a `FOR SHARE` Mission lock, stable ordered source/target Task locks, and rejects a missing `actionable` field instead of converting it to FYI. Concurrent submission and pause tests verify their event ordering against sends. Read-only re-review of `aa0a53e..6df7d54` found zero remaining Critical, Important, or Minor findings. No model egress, QQ send, external MCP/GitHub call, business account, or production action was performed.

## Remaining boundary

The fixed peer pair has a bidirectional Kernel lifecycle and the current product Worker adapter now exposes the same lifecycle only in an unqualified offline Fake Runtime surface. Real provider qualification, Worker dispatch to other fixed roles, and the remaining REQ-02/REQ-13 work stay open. The rest of the finite scope remains tracked in `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`.
