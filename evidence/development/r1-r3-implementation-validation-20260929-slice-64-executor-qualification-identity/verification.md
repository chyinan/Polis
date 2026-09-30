# Slice 64 — Executor qualification identity and owner decision path

Date: 2026-09-29  
Schema: 48

## Changes

- Added append-only migration 48 to separate the running executor binary, host build, isolation policy, and Node toolchain fingerprints. Qualification is valid only for the exact tuple on the current host; a revocation remains a barrier across host identities.
- Added stable host/executor fingerprint collection for Windows and Linux. Windows uses OS build and UBR registry values; Linux uses the release identity and kernel release. Unsupported hosts fail closed.
- Added a bounded `polis-node-executor-qualification@1` report. It requires the profile's fixed check set and same-company, same-MissionInput revision and digest references. The owner decision path verifies every referenced CAS object before appending a qualification or revocation event.
- Added a token-authenticated Workbench command and read projection for the qualification decision. The UI shows the current executor, host, isolation-policy and toolchain identity, evidence report digest, stale-identity state, and the separate qualify/revoke actions.
- Kept Windows service ingress and hard per-Job CPU, memory and storage bounds as explicit unmet qualification checks. The API cannot turn missing host proof into qualification.
- Corrected recovery test fixtures to use valid IDs for versioned profiles, and changed the startup test to assert that global recovery can reconcile one or more unresolved Jobs in the dedicated test database.

## Verification

- `bash scripts/r1-capability-source-postgres-test.sh` — passed on disposable PostgreSQL 18 through Schema 48. This includes current CAS-bound qualification, preparation gate/revocation, cross-profile isolation, Windows startup recovery, Task input Worker-to-Workbench readback and command-route coverage.
- `bash scripts/r2-cross-backend-handover-postgres-test.sh` — passed; the schema-39 handover migration round-trip, migration evidence guard and cross-backend handover flows passed.
- `bash scripts/r2-recovery-backup-postgres-test.sh` — passed on dedicated source and restore databases.
- `bash scripts/r3-domain-evidence-postgres-test.sh` — passed for the content/research evidence submission and review contracts on disposable PostgreSQL. This is contract evidence only; neither profile is domain-qualified.
- `bash scripts/go.sh test ./... -count=1` — passed. Database-backed tests without `POLIS_TEST_DSN` skipped under their existing contract; the disposable scripts above ran the targeted PostgreSQL cases.
- Frontend: Vitest 108/108, TypeScript build check, ESLint with zero warnings, and Vite production build passed. Vite reports the existing large-chunk advisory.
- Linux amd64 and Windows amd64 `cmd/...` builds passed. These are builds; the Windows host APIs, AppContainer, Node/npm, browser ingress, and clean-VM installer were not run on a native Windows host.
- No real model turn, QQ send, external MCP/GitHub call, business account, or production database was used.

## Qualification boundary

The owner API now provides an auditable decision path, not automatic proof. The synthetic report fixtures used by PostgreSQL tests do not qualify the current Windows or Linux host. Host-specific isolation, ingress, resource limits, native Node/npm behavior, browser validation, clean-host recovery, packaged Desktop install/update, and signature verification remain unqualified. R3 content-operations and research profiles remain `not_run` until separate real-domain quality, recovery, cost, intervention, and organization-benefit evidence is reviewed.
