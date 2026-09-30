# Slice 25 review fixes

Date: 2026-09-25

## Findings addressed

1. **Git clean-filter boundary:** removed the `git status` worktree scan from `ImportGitCommit`. The importer now reads only repository-root metadata, commit/tree objects, and tracked blob objects using built-in `rev-parse`, `cat-file`, and `ls-tree` commands. It never inspects worktree file contents. Every snapshot carries `workingTreeStatus=not_included_by_policy`, is marked partial, and directs the operator to add a separate directory snapshot for local changes. A temporary repository with `.gitattributes` and a configured local clean filter was used; the importer left its marker absent. A configured unreachable remote also remained untouched.
2. **Linux policy metadata parity:** frontend validation now requires the same one-to-eight normalized registry hosts as Go's `CanonicalizeLinuxNodeEnvironmentPolicy`, while retaining the Linux offline install policy and `networkPolicy=deny_all`. Regression coverage accepts a valid host list and rejects an empty list or a registry allowlist network policy. The environment page displays `deny_all` separately from the registry metadata.
3. **CLI source-note wording:** corrected the partial-import warning to say working-tree files were not read or included and to direct operators to add a separate directory snapshot for local changes.

## Verification

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test ./internal/intake ./internal/control ./cmd/polis -count=1` | PASS after the status-scan removal. |
| Dedicated PostgreSQL 18.6 / schema 28: `POLIS_TEST_DSN='<review-fix runtime DSN>' bash scripts/go.sh test ./internal/control -run '^TestProductWorkerReceivesFrozenGitSnapshotFromSelectedCommit$' -count=1` | PASS. Fake Worker received commit source/provenance, no working-tree bytes, and two refs with `provider_egress=0`. |
| Dedicated PostgreSQL race run: `POLIS_TEST_DSN='<review-fix runtime DSN>' bash scripts/go.sh test -race ./internal/intake ./internal/control -run Git -count=1` | PASS. |
| `bash scripts/go.sh test ./... -count=1` | PASS. |
| `bash scripts/go.sh test ./cmd/polis -count=1` after the CLI message correction | PASS. |
| Frontend `npm test`, `npm run typecheck`, `npm run lint`, `npm run build` | PASS; 66 tests. Existing >500 kB chunk advisory remains. |
| Linux `polis` build and Windows amd64 `polis`/`polisd` builds after the CLI message correction | PASS. |
| `git diff --check` | PASS. |

The review-fix PostgreSQL instance used a dedicated `polis_r0_r1_git_s25_fix` database, migrated through schema 28, and was stopped; its exact `/tmp/polis-r1-s25fix.*` root was removed. Setup, migration, Worker, race and cleanup logs remain in this evidence directory. No model, remote Git request, GitHub account, external MCP, QQ, or project script ran.

## Read-only re-review

The final read-only review reported Critical 0, Important 0, Minor 0. It confirmed the importer no longer calls `git status`, the Linux profile validator matches the backend's 1–8 normalized registry-host contract while retaining `deny_all`, the CLI warning matches the policy, and the Git snapshot still verifies across CAS, frozen Task manifest, Worker receipt, and Workbench projection. The reviewer did not modify files or run tests/builds/network calls.
