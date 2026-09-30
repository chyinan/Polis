# Slice 68 — Bounded Windows Node workspace volume

Date: 2026-09-29  
Schema: 50 (no migration)  
Status: source implementation and local verification passed; Windows storage isolation remains unqualified.

## Implemented

- `POLIS_WINDOWS_NODE_WORKSPACE_ROOT` is now required when the Windows Node/npm executor is enabled.
- Environment and Runner independently inspect the configured volume. They require a direct-child workspace directory on an NTFS volume labeled `POLIS_WORKSPACE`, with 1–16 GiB logical capacity, free space, a valid volume GUID, and a volume identity different from the control executable and Windows system volumes.
- Control captures the authorized workspace policy fingerprint before and after the authorization transaction, places the stable digest on the execution snapshot, and refuses preparation if it changed. Environment checks that digest against the observed volume; Runner rechecks the volume GUID, serial, label, filesystem and capacity before creating the workspace directory.
- Runner creates and addresses the unique per-AppContainer directory through the stable volume-GUID path rather than re-resolving the configured drive letter. This prevents a drive-letter remount from redirecting the workspace write after validation. The package SID receives bounded read/write/execute access with protected ACL inheritance; cleanup access remains with the control user, SYSTEM and Administrators.
- For bounded-storage AppContainers, Runner walks the OS-managed AppContainer LocalAppData profile and applies a protected DACL that denies package-SID write/create/delete rights while retaining read/execute and trusted cleanup access. The explicit LOCALAPPDATA/TEMP/TMP variables point to the bounded workspace volume.
- Cleanup happens after the AppContainer process tree drains and its profile is deleted. Constructor failures retry deletion three times and report any residual path; normal `Close` retains cleanup ownership for retry.
- If the first workspace-ACL cleanup retries still leave a directory, the residual root is retained in a partial backend. Registry sandbox construction, Environment snapshot cleanup and Control pending-cleanup preserve the same owner for another attempt.
- Materialized project files, staged Node/npm and the explicit HOME, LOCALAPPDATA, TEMP and TMP paths use the configured volume. A capacity-limited volume is a hard **aggregate** ceiling shared by all AppContainers; this does not provide an independent per-job quota.
- The Windows Node isolation policy fingerprint advances to `windows-appcontainer-node-policy@4`, invalidating qualification decisions made against the prior storage policy.
- On Windows, the live isolation fingerprint binds the configured root, volume GUID, serial, label, filesystem and logical capacity. Moving to another volume or changing its capacity requires fresh qualification; volatile free-space changes do not alter the fingerprint.
- Kernel rechecks that binding before qualification-sensitive operations. Control compares the policy fingerprint before and after the authorization transaction, then passes the authorized digest into preparation. Environment and Runner validate it again immediately before setup.
- The workspace is created through the verified volume-GUID path, so replacing the configured drive-letter mount cannot redirect the write between validation and directory creation.
- Cleanup tests verify transient retry and persistent residual-path reporting. Normal AppContainer close retains the cleanup flag so a caller can retry `Close`.
- The product does not create or attach a VHD, request volume-management privileges, install WFP filters or change the loopback list. Operators must pre-provision the volume.

## Verification

- The pure storage policy test first failed because the validator did not exist. It passed after implementation and rejects same control/system volumes, wrong label or filesystem, undersized/oversized capacity, missing free space, invalid serials, out-of-volume/nested roots and available bytes above total capacity.
- The policy fingerprint test first failed against `@3`, then passed with the new `@4` revision.
- The storage-binding fingerprint test covers changed volume serial/capacity and confirms a free-space-only change does not cause identity churn.
- A Windows-native kernel test first failed against the startup-cache behavior; it passed after live volume revalidation with the configured workspace root intentionally unavailable.
- The Windows-native Environment test verifies the volume-GUID root is absolute and a child workspace path resolves beneath it.
- A filtered native Windows AppContainer test started a local helper process and attempted to create a file directly under the OS-managed path returned by `GetAppContainerFolderPath`; the kernel returned `ERROR_ACCESS_DENIED`, and the test profile was deleted on `Close`.
- Cleanup tests passed for transient retry and persistent residual-path reporting.
- `bash scripts/go.sh test ./... -count=1` passed.
- After the final residual-owner propagation fix, `bash scripts/go.sh test ./internal/runner -count=1` and the Windows amd64 Runner test-binary cross-compile passed.
- `bash scripts/go.sh build ./cmd/...` passed on Linux.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` passed.
- Windows amd64 `cmd/...` and `internal/runner`, `internal/environment` and `internal/kernel` test binaries cross-compiled.
- Four filtered Windows-native tests passed: `TestWindowsAppContainerDefaultProfileWriteIsDenied`, `TestInspectWindowsNodeWorkspaceStorageRejectsTheControlVolume`, `TestPrepareWindowsBoundedWorkspaceRejectsMissingBinding`, and `TestCurrentWindowsExecutorFingerprintDoesNotReuseStartupVolumeIdentity`. The ACL probe created and deleted only a temporary AppContainer profile; the other tests did not provision a volume or create an AppContainer. No test ran Node/npm or altered WFP/loopback system policy.
- Test binaries were removed after execution. No PostgreSQL migration or database operation was needed.

## Qualification limits

No `POLIS_WORKSPACE` volume was provisioned or written to. The package-SID default-profile write denial passed one isolated native probe, but ACL inheritance and access across a volume-bound worker, full-volume behavior, alternate AppContainer storage APIs, WFP/registry policy, real Node/npm installation, project execution, cancellation under disk pressure, fixed-port service ingress and clean-VM behavior remain unverified. The R1 executor remains gated by the existing owner qualification flow.
