# Slice 58 verification — Linux/Node instance ownership and Worker inputs

Date: 2026-09-28

Schema: 44. No migration was added.

## Results

- `rtk bash scripts/go.sh test ./... -count=1` — PASS.
- `rtk bash scripts/go.sh build ./cmd/...` — PASS.
- `rtk bash -c "export GOOS=windows GOARCH=amd64 && bash scripts/go.sh build ./cmd/..."` — PASS.
- `rtk bash scripts/r1-capability-source-postgres-test.sh` — PASS at Schema 44. Fake Worker turns received frozen text, image, directory, Git, PDF, CSV and ZIP input; final receipts matched the frozen manifest and content digests with provider egress 0.
- `rtk bash scripts/r2-cross-backend-handover-postgres-test.sh` — PASS.
- `rtk bash scripts/r2-recovery-backup-postgres-test.sh` — PASS (`R2_RECOVERY_BACKUP_RESTORE=PASSED`).
- `rtk bash scripts/r3-domain-evidence-postgres-test.sh` — PASS.
- `rtk bash scripts/go.sh test ./cmd/polis -run TestLinuxNodeInstanceIdentity -count=1` — PASS. It covers server host/port, database, role, BlobRoot, password rotation, case-sensitive socket paths, DNS case normalization, and equivalent BlobRoot paths.
- Targeted Linux tests for runtime/cgroup path containment, durable runtime identity, exclusive root leases, owner markers, unknown/linked children, startup root requirements and active-profile recovery — PASS.

## Boundaries

The runtime identity is a SHA-256 digest over the PostgreSQL server host/port, database name, database role and canonical BlobRoot. It excludes the password so routine password rotation keeps the installation identity stable. Tests confirm a different host, port, database or BlobRoot receives a different identity, preserve case-sensitive Unix-socket paths, normalize DNS host case, and accept equivalent BlobRoot paths. The runtime directory marker and both directory leases are local OS state. The cgroup owner child binds the mounted cgroup root to that identity; it remains empty and is preserved while only per-job `polis-node-<24 lowercase hex>` cgroups are cleaned. Legacy version-1 markers fail closed; `docs/implementation/R2_LINUX_NODE_FOUNDATION.md` documents the manual migration preconditions.

No Linux delegated cgroup v2 host, bubblewrap process, Node/npm install, project script, native Windows AppContainer/WFP, real model turn, QQ send, MCP command/endpoint, GitHub account request, or production workload was run. The R3 PostgreSQL checks validate evidence submission and review workflows; both reference domains remain `not_run` until content-operations and research evidence is independently supplied and reviewed.
