# Slice 49 — Windows JobRun host reconciliation

Date: 2026-09-27

## Scope

Project JobRuns in the Windows Node/Node.js AppContainer executor now have a stable named Job Object derived from the durable JobRun ID. `PROC_THREAD_ATTRIBUTE_JOB_LIST` assigns the suspended child at process creation. Normal root exit and Stop close the root handle, terminate remaining members, and wait for the active-process count to reach zero before signaling cleanup completion or issuing `StopProof`. A timeout/query/termination failure remains visible as an unknown stop and retains snapshot ownership. Restart reconciliation verifies the Job Object's kill-on-close and active-process policies, terminates remaining members and waits for the active-process count to reach zero. It returns an identity-bound proof that the tracked wrapper uses to release retained ownership. An absent named object is accepted as stopped under the Windows kill-on-close lifetime rule.

## Verification

- `rtk proxy bash scripts/go.sh test -count=1 ./internal/runner ./internal/environment ./internal/control` — passed: runner, environment and control packages.
- Windows amd64 test-binary cross-compilation for `./internal/runner`, `./internal/environment` and `./internal/control` — passed.
- Native Windows `TestWindowsReconcileNamedJobTerminatesItsActiveTree` — passed. It launched only a local sleeping test-helper process, reopened the named Job Object, terminated it, and waited for process-tree accounting to reach zero.
- Native Windows `TestWindowsAppContainerStopWaitsForTreeCleanupAfterRootExit` and `TestWindowsAppContainerDoesNotProveStopWhenTreeCleanupIsUnknown` — passed; both use synthetic process wrappers and do not create an AppContainer.
- `rtk git diff --check` — passed.

No AppContainer was created, no Node/npm or project script ran, no WFP filter or loopback SID list was changed, and no provider Worker, QQ bot or MCP server/endpoint was started. This evidence qualifies the local Job Object recovery primitive and the no-false-proof process logic only; it is not Windows/Node release qualification. Provider `runner.Process` WorkerSession recovery remains open.
