# R0.5B13 provider launch investigation

## Confirmed B11/B12 composition

- B11 local preflight (`cmd/polis-r05b11/main.go`) resolves the verified manifest, materializes `NativeArgsWithTransportPolicy`, calls `runner.Start` directly, then creates the protocol client and runs initialize/thread-start.
- B12 diagnostic (`cmd/polis-r05b10/main.go` with B12 environment overrides) calls `CodexRuntime.Start`. That path performs diagnostic reservation/start fencing, readiness rebinding, session-home/evidence checks, credential snapshot materialization, `NativeArgsWithTransportPolicy`, `runner.Start`, and `codex.NewWithModelAndVersion`.
- Static binary/helper paths, hashes, argv (`codex.exe app-server --stdio`), HOME/CODEX_HOME derivation and Windows native launch mode are intended to be the same. B12 reuses the B11 runtime root and provider session ID under the explicit `allowQualifiedB11PreflightSessionReuse` exception, while B11 direct preflight and B12 production composition do not share one launch-spec object.

## Frozen B12 evidence limitation

`provider-transport-terminal.json` records `process_start_requested_at`, no PID, no process-start timestamp, no protocol messages, `protocol_evidence_error=true`, and only `turn_terminal=process_start_failed`. The runner discards the `CodexRuntime.Start` error, so the frozen evidence cannot distinguish `cmd.Start`, credential/evidence setup, protocol client creation, or an early child exit. No OS error code or safe error text was persisted.

## Relevant implementation gaps

- `runner.NativeEnvironment` starts from the complete parent `os.Environ()` and removes selected names, leaving arbitrary `PATH`, `CODEX_VERSION`, `USERPROFILE`, `TEMP/TMP`, and unrelated runtime variables available to the child.
- `runner.NativeLaunchEnvelope` fingerprints only normalized argv, binary/helper hashes, abstract HOME/CODEX_HOME, stdio text, boundary and transport policy. It does not capture controlled environment policy/materialized safe entries, runtime/evidence directory derivation, process flags or pipe setup.
- `provider.CodexRuntime.Start` returns raw errors from multiple launch phases and callers classify all such failures as `process_start_failed`.

## Final root-cause classification

The frozen B12 observed category is `PROCESS_CREATE_FAILED_NO_PID`. The dominant root cause is `REPORTING_CLASSIFICATION_DEFECT`: the terminal proves no PID, no process-start timestamp, no protocol messages and only `process_start_failed`, but contains neither an OS error code nor safe error text. Therefore the immutable record cannot truthfully be narrowed to executable missing, access denied, cwd failure or early child exit. The repaired launcher now records the exact phase/reason/OS code/PID state for future failures; local production qualification reproduces the canonical envelope 3/3 and the adapter handshake without provider traffic.
