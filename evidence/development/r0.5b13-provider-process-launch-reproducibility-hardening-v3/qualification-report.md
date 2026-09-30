# R0.5B13 provider process launch reproducibility hardening

## Result

`r0_5b13_provider_process_launch_reproducibility = PASSED` for local/offline scope.

No provider reservation, provider egress, turn, live canary or LIVE_3 was run. The machine-readable result is `qualification.json`; the B11/B12 launch comparison is `b11-b12-launch-diff.json`.

## B12 frozen failure classification

The frozen B12 observed category is `PROCESS_CREATE_FAILED_NO_PID`: the runner persisted no PID, no process-start timestamp and no protocol messages. The dominant root cause is `REPORTING_CLASSIFICATION_DEFECT`, because the immutable record retained only `process_start_failed`, `protocol_evidence_error=true`, and neither an OS error code nor safe error text. An executable/access/cwd error cannot be truthfully inferred from that record. The remediation now preserves phase, reason code, OS error code when available, process-created and PID-present fields.

## B11/B12 comparison and root cause

B11 used `cmd/polis-r05b11 -> NativeArgsWithTransportPolicy -> runner.Start`; B12 used the production `CodexRuntime.Start` composition. The canonical comparison shows the same verified executable/helper, argv, controlled environment, stdio, process flags, runtime-home derivation and process policy. The differences are the launcher/lifecycle composition and B12's separate evidence root. The dominant root cause is therefore `REPORTING_CLASSIFICATION_DEFECT`, with the prior split launch composition and unmodeled lifecycle inputs as the reproducibility defect.

## Current canonical launch contract

- envelope schema: `r03a-native-launch-envelope@2`
- process spec schema: `polis-process-launch-spec@1`
- environment: `controlled-runtime-allowlist@1`; ambient inheritance denied
- Windows environment: dedicated `HOME`, `CODEX_HOME`, `USERPROFILE`, `TEMP`, `TMP`; controlled `SystemRoot`, `WINDIR`, and derived `PATH`; no ambient `CODEX_VERSION`, `POLIS_*`, proxy or credential variables
- stdio: separate stdin/stdout/stderr pipes; Windows hidden-window process handle; no job object; clean stop is handle-based
- runtime directories: dedicated runtime root/session home, `runtime_home/tmp`, and dedicated evidence root; resolved paths are evidence, not durable fingerprint identity

## Repeated local production-path qualification

The v3 evidence records 3 independent cycles, each with process start, initialize, exact 7-tool thread/start registration, clean stop, and zero orphan processes. All three used runtime `0.154.0-alpha.6.2`, executable SHA `081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575`, helper SHA `fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e`, and provider egress `0`. An additional `RealProviderWorkerAdapter` local no-turn handshake also passed.

## Current identity

- surface: `polis-product-tool-surface@4`, 7 tools
- manifest: `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`
- schema: `2206` bytes / `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`
- current launch envelope fingerprint: `dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d`
- current exact-surface execution fingerprint: `e736d8298f0920c6dd81796006b2486881647f0fdbc66bb111bbd2387a1e99ff`
- provider surface changed: `false`
- runtime binding changed: `false`
- launch envelope changed: `true`
- exact-surface execution fingerprint changed: `true`

`eligible_for_new_v4_live_qualification = YES` is an eligibility result only. No live qualification is started by this task.
