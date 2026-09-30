# R1–R3 continuation verification — 2026-09-24

Workspace: isolated worktree `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`.

## Passed

- Full offline Go suite `go test ./... -count=1` passed across all packages after Schema 20 and Control/Workbench changes with `POLIS_TEST_DSN` and `POLIS_DSN` unset. PostgreSQL integration tests were skipped in this run.
- `go test ./internal/intake ./internal/control ./internal/environment ./internal/runner ./internal/workbench ./internal/desktop -count=1` without a database: all six packages passed. PostgreSQL-backed tests were skipped in this invocation.
- On the dedicated PostgreSQL 18 database `polis_r0_r1r3_cleanverify_20260924` (migrations 1–18), these isolated tests passed:
  - `TestProductWorkerReceivesFrozenTextInputsInTurnPrompt`
  - `TestProductWorkerReceivesTextFilesFromFrozenDirectoryInput`
  - `TestTaskInputManifestReadShowsFinalWorkerDeliveryStatus`
  - `TestTaskInputManifestReadProjectsPerFileDirectoryDeliveryReceipt`
  - `TestCapabilityApprovalBindingAndRevocationStayVersionPinned`
  - `TestControlledStdioMCPApprovalAndEmployeeUnbindAreAudited`
- `TestQualifiedArtifactDeliveryManifestAndPackageReadFromPostgresAndCAS` passed on the same dedicated Schema 18 database; it created a fake qualified Artifact, read its DeliveryManifest from PostgreSQL/CAS, and checked the emitted ZIP entry hashes.
- The dedicated PostgreSQL 18 database `polis_r0_envjobs_20260924` migrated from 1 through 20. On it, `TestEnvironmentPreparationPersistsPolicyAndIsolationBlocksIdempotently` passed, including immutable-revision rejection, idempotent ensure receipts, policy/executor blocking, and denial of JobRun creation without qualified isolation.
- On the same Schema 20 database, `TestEnvironmentLifecycleControlCommandsPersistGatedStatus` passed. `TestEnvironmentReadStoreShowsPolicyAndExecutorQualificationGates` passed with a seeded JobRun/service generation and verified that an expired lease projects as unhealthy.
- Goose applied migrations 00019 and 00020; Schema 20 requires a lease for active service endpoints.
- Windows amd64 `internal/runner` and `internal/environment` test binaries were built and executed on the local Windows host; each returned `PASS`. Runner coverage included terminating a child process when its root exits.
- Frontend `npm test`: 46 tests passed, including successful and tampered ZIP download verification plus scoped environment/JobRun parsing and policy/ensure requests. `npm run typecheck`, `npm run lint`, and `npm run build` exited 0.
- `go build ./cmd/...` and `GOOS=windows GOARCH=amd64 go build ./cmd/...` exited 0 after the Schema 20 changes.
- Artifact DeliveryManifest package-builder and desktop session-token path unit tests passed. Full browser download through desktop session middleware remains unverified end-to-end.

## Not passed as a complete suite

The combined PostgreSQL-backed run `go test ./internal/intake ./internal/control ./internal/environment ./internal/runner ./internal/workbench ./internal/desktop -count=1` on the accumulated verification database failed five `internal/control` tests at `kernel.Open` with `POLICY_DENIED`; intake, environment, runner, workbench and desktop passed. The selected new Worker, readback and capability tests above passed separately on the same dedicated database. Do not report the combined run as green. Broad database suites need isolated state or database reset between test groups; do not reset or delete this evidence database as a shortcut.

## External actions and unverified gates

- No real model turn, QQ send, external MCP call, GitHub account access, production action, or dependency installation was run.
- The Node/npm code only inspects manifests, lockfiles, sources and produces fixed install argv. It does not execute npm.
- Process tree containment is verified; filesystem/network isolation is not implemented.
- Artifact package generation from PostgreSQL/CAS and React API integrity validation were verified separately; browser UI plus desktop token middleware was not exercised together.
- CSV parsing and Worker-context tests are local; PDF, ZIP, R2 profile qualification, R2 E-START, and R3 content/research domain qualification remain unrun.
