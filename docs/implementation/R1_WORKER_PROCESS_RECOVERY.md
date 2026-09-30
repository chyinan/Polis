# R1 Windows Worker process recovery foundation

Updated: 2026-09-27. Schema 40; no migration added.

## Process containment

Windows `runner.Process` now creates a globally named Job Object from the persisted WorkerSession ID, allowing a restarted desktop process in another Windows logon session to find the same object. It launches the process suspended and uses `PROC_THREAD_ATTRIBUTE_JOB_LIST` so membership is assigned during `CreateProcess`, before the main thread can run. The Job Object has kill-on-close and a 64-process limit. Stopping a Worker terminates the Job Object and waits for the root and descendants before `StopProof` is returned. The launcher uses an explicit inherited-handle list for stdin/stdout/stderr. The default Job Object ACL restricts access to the creating Windows account; starting recovery under another account fails closed.

Each WorkerSession stores its host OS and containment profile as an append-only `worker_observations` record in the same database transaction that creates the session. The Control adapters also bind this profile immediately before starting the process; process attachment verifies the same profile. This makes a crash between process creation and PID attachment recoverable by the durable session ID. No historical schema or evidence was rewritten.

Stop handling retains the adapter's process/session ownership when the database transition, process cleanup, no-process finalization, or reservation close fails, including Provider and deterministic startup failures before activation. A later Stop retries the remaining steps; process attachment and database stop confirmation are idempotent for the same session/PID proof. If the database committed a stop but its response was lost, adapters replay the saved proof before attempting process attachment. Host-reconciliation receipts are also checked after a stopped-session replay so a lost response does not strand its owner. Deterministic launch failures with `ProcessCreated` keep a host-only owner until the named Job is reaped. Local zero-turn qualification processes remain owned until a PID-bound stop proof is confirmed. Windows receives an explicit empty environment when no allowlisted entries were supplied, so `CreateProcess` cannot inherit the Polis parent environment. Bounded `Run` returns exit and cleanup failures on normal completion, timeout, and output-limit exits, and `WaitError` preserves both a nonzero exit code and an unconfirmed tree stop.

## Startup recovery

After `kernel.Open` has fenced old sessions as `reconcile_required`, Windows `polis serve` asks the Kernel to enumerate only sessions whose persisted host/profile identify the Windows named Job Object. For each session, the host reaper opens the Job Object by the exact session-derived name, checks its kill-on-close and process-limit policy, terminates any remaining members, and waits for the active-process count to reach zero. The returned proof binds the session ID and persisted PID. The Kernel rechecks the session snapshot under the company write guard before marking it stopped and appending the reconciliation evidence.

The reaper stops OS processes only. It does not retry a Worker start or reset its Task to ready; an unknown Mission start remains fenced, so business effects are not replayed. If host verification fails, the session stays unresolved and Workbench startup continues with the other company surfaces available.

The Worker launcher uses Microsoft's [`PROC_THREAD_ATTRIBUTE_JOB_LIST`](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute), available on Windows 10 version 1703 and later (Windows Server 2016 version 1709 and later). Microsoft's [Kernel Object Namespaces](https://learn.microsoft.com/en-us/windows/win32/termserv/kernel-object-namespaces) documents that Job Objects can use the global namespace across Terminal Services sessions. The stop proof is based on active-process accounting described in Microsoft's [Job Objects documentation](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects).

## Limits

- Sessions created before process-containment metadata was introduced remain `reconcile_required`; they are not guessed to be Windows sessions from a PID.
- Linux records its process-group profile but has no automatic restart reaper yet. Those sessions remain unresolved.
- This verifies process-tree containment with a local helper process. It does not run a real model turn or qualify Codex credentials, provider transport, Windows AppContainer isolation, or a release build.

## Verification

Focused Linux runner/kernel/control/Workbench command tests, Windows amd64 runner/kernel/control test-binary cross-compiles, and a native Windows helper-process Job Object recovery test pass. The dedicated PostgreSQL 18 script applies the full Schema 40 migration chain and verifies host binding immutability, recovery candidate readback, and denial of a fabricated stop proof. The native Windows helper uses the test binary only; it does not start a Worker or call a provider. Evidence: `evidence/development/r1-r3-implementation-validation-20260927-slice-50-worker-process-reconciliation/verification.md`.
