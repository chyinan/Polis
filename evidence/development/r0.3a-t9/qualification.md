# R0.3A-T9 No-Injected-Proxy Minimal Transport Canary

Result: `INCONCLUSIVE`.

The preflight passed and established a single intended execution change from T6: Polis did not inject `HTTP_PROXY` or `HTTPS_PROXY` into the child. `POLIS_NATIVE_PROXY`, `ALL_PROXY`, and `NO_PROXY` were also absent. The pinned Codex/app-server version, binary, model, effort, WSL runtime, sandbox, auth source and opaque auth fingerprint, code-mode-host, native protocol, HOME/CODEX_HOME behavior, invocation path, WebSocket policy, zero-tool surface, prompt, developer instruction, and deadlines matched the T6 baseline.

The one authorized `gpt-5.6-luna / medium` turn established a new thread and started a turn. It produced the user-message item, then emitted three structured `responseStreamDisconnected` events with `willRetry=true`, all before first valid model output. No first valid output, usage update, tool call, or `turn/completed` event occurred. The phase-aware first-output deadline ended the turn after `1m30.003185659s`; process-group termination was confirmed. Total experiment elapsed time was 106,788 ms.

The experimental result remains epistemically `inconclusive`. Scheduling state for the new no-injected-proxy L1 combination is `unqualified_for_business_execution`; R0.3A business execution remains blocked. This result shows that injected localhost HTTP(S) proxy is not necessary for the observed pre-output reconnect/no-output failure. It does not prove that the proxy has no effect, that it caused no historical failures, or that the provider is permanently unavailable.

The old T6/T7 combination remains unchanged at key `58c8d49d9545d179820f03a4505163970bcaf461d6720fd3541640a5113cba45`. The new no-proxy combination is `ca04fb2fe5004a7c14b8ccbae7bc7b043d010ad517c571c1c04136415c20b782`.

The frozen T7 record contains an earlier metadata serialization defect in its embedded combination fields; its declared key is nevertheless exactly reproduced from the frozen T6 readiness manifest. T9 records this as `t7_embedded_combination_matches_declared_key=false` and does not alter T7 evidence.

Exact live command:

```bash
rtk proxy wsl -d Ubuntu-22.04 -- bash scripts/r03a-t9-canary.sh
```

No retry, reset, Backend, Frontend, successor, Reviewer, High turn, dynamic business tool, or business writer was used.

Post-run local verification passed:

```bash
bash scripts/go.sh test ./... -count=1
bash scripts/go.sh test -race ./internal/codex ./internal/probe -count=1
bash scripts/go.sh vet ./...
bash scripts/go.sh build ./cmd/...
git diff --check
```
