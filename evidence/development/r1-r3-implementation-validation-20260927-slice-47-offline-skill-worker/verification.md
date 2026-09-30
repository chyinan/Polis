# Slice 47: offline read-only Skill Worker path

Date: 2026-09-27
Schema: 40 (no migration added)

## Result

The exact `polis-product-tool-surface@5` Skill-load path now runs only in the deterministic fake runtime when `ReadOnlySkillSurface` is explicitly enabled. The fake Worker reads its permission-filtered catalog, loads the first listed `SKILL.md` through the existing `EmployeeTools` and Kernel authorization path, checks the static-text/no-extra-permissions response boundary, and leaves an append-only reference event with Skill ID, version digest, and path. Skill body bytes are not copied into the event.

The real provider authorization remains pinned to v4. The fake-only profile carries the explicit `offline-skill-surface-unqualified` marker; it does not alter capability qualification state, which remains `runtime_unqualified`.

## Verification

- `scripts/r1-capability-source-postgres-test.sh` passed on its dedicated disposable PostgreSQL 18 cluster. It staged the database at Schema 39 for historical migration tests, advanced to Schema 40, then passed the capability Kernel race tests, the new end-to-end Worker test, and selected Workbench capability/Manifest tests.
- `scripts/go.sh test ./...` passed after the pinned v5 digest changes. The Kernel package completed in 43.950 seconds; all listed packages passed.
- `scripts/go.sh test ./internal/provider -count=1` passed after hardening the fake-only gate to recompute the actual v5 tool-array and aggregate schema digests. Its mutation regression first failed when a live in-memory tool name changed, then passed once the gate rejected the drift.
- `scripts/go.sh build ./cmd/...` passed for the Linux host.
- Windows amd64 cross-builds passed for `./cmd/polis` and `./cmd/polisd`, with outputs written under `/tmp`.
- `git diff --check` passed.
- The end-to-end Skill Worker test confirmed one exact Skill-use event, no `content` field in that event, successful Task delivery, and provider egress `0`.

## Boundaries

No real model turn, Skill script, MCP process/endpoint, QQ send, GitHub account, production database, or Windows AppContainer/WFP setup was used. This verifies the local Worker-to-Kernel authorization and content path; it does not qualify a real provider v5 turn or prove that a model applied the guidance.

## Review

The read-only code review identified two P3 items: stale "Latest" labels on historical handoff entries and the lack of production-pinned v5 schema digests. Historical entries now say "Previous"; v5 manifest/schema digests and byte count are pinned in the authorization gate and asserted by the provider surface test. The reviewer rechecked both changes and reported no remaining or new findings.

R1/R2/R3 work remains in progress. Host-confirmed WorkerSession recovery, migration-attempt failure journaling, R2 multi-day/cutover/upgrade rollback and Linux host qualification, plus independent real-domain R3 qualification, remain open.
