# R0.3A-T4 Known-Good vs Failing Native Execution Differential

Status: diagnostic qualification complete. No Medium, High, reset, retry, or real provider call was used.

## Compared runs

Known-good:

- R0.2 Luna Medium turn 1
- R0.2 Luna Medium successor turn 2
- R0.2H3 successful High reviewer (auxiliary)

Failing:

- first R0.3A Backend
- R0.3A-T2 Backend
- R0.3A-T3 Backend

Normalized traces and the full machine-readable matrix are in [normalized-diff.json](D:/Programs/Polis/evidence/development/r0.3a-t4/normalized-diff.json) and [normalized-diff.md](D:/Programs/Polis/evidence/development/r0.3a-t4/normalized-diff.md).

Historical traces were not edited. Historical protocol files do not contain a monotonic clock; traces therefore preserve audit wall/emitted deltas and explicitly mark `monotonic_delta_ms=not_recorded`.

## First evidence-supported divergence

The native runtime combination is not wholly different:

- Codex CLI/app-server: `0.151.0` on known-good and failing records;
- model: `gpt-5.6-luna`;
- Medium effort: same for R0.2 Medium and all failing Backend runs;
- cwd: `/work`;
- sandbox: `read-only`;
- recorded binary SHA: same for R0.2 and the first R0.3A preflight.

The first confirmed differences before business tool execution are the registered Backend tool surface and request payload:

- known-good Medium: 7 tools, schema digest `d0886c0e...`, schema bytes 2438;
- failing Backend: 11 tools, schema digest `b36063bf...`, schema bytes 3277;
- known-good developer instruction bytes: 286;
- failing Backend developer instruction bytes: 185;
- known-good thread/start request bytes: 2975;
- failing Backend thread/start request bytes: 3717.

The failing Backend tool set is the R0.3A peer-worker surface, so this is a confirmed execution-combination difference, not evidence that the peer business fixture is wrong. It is not yet proof that the larger tool surface caused the provider disconnect.

The first post-`turn/started` behavior also diverges:

- known-good runs proceed through reasoning/agent output and then tool activity;
- all three failing Backend runs show user-message item events followed by structured `responseStreamDisconnected` with `willRetry=true`, before any valid agent output, usage update, or Polis tool call.

## Not recorded / not inferred

The historical runs do not provide enough evidence for these dimensions:

- per-turn proxy URL/implementation/configuration;
- per-run code-mode-host SHA for failing T2/T3;
- per-turn callback-host readiness and employee binding readiness;
- process tree and local IPC endpoint;
- attachments/resources and inherited context manifest sizes;
- true monotonic elapsed timestamps.

These remain `not_recorded`, not assumed equal or different. The first failing preflight recorded `native_protocol=passed_without_inference`; that is a preflight fact, not proof of per-turn callback readiness.

## Historical state

- deterministic R0.3A: `PASSED`;
- first real R0.3A: `INCONCLUSIVE`;
- T1: `PASSED`;
- T2: `INCONCLUSIVE`;
- T2.1: `PASSED`;
- T3: `INCONCLUSIVE`;
- parent ProblemKey: `r03a-real-peer-collaboration-v1`.

All allowances remain sealed in their original boundaries. No historical evidence or hash was modified.

## Minimal next diagnostic path

Because a concrete divergence exists (peer Backend tool registration/request shape), the next step should be a pure-local comparison/canary of the exact native request and tool binding before another business Backend turn. If that local comparison cannot establish a fault and a transport canary is still needed, use one separately authorized no-business-side-effect Medium with the exact T3 binary/proxy/model profile and minimal or no dynamic tools. Do not resume the sealed T3 allowance or start Frontend.
