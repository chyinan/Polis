# R0.3A-T8 Working Codex vs Polis Native Execution Path Differential

Status: local differential audit complete. No Medium, High, reset, Backend, Frontend, Reviewer, or provider turn was used.

## Compared paths

- A: current working Windows Codex desktop/CLI path. The npm `codex` wrapper resolves to `C:\Users\chyinan\AppData\Roaming\npm\codex.ps1`; the running binary is `C:\Users\chyinan\AppData\Local\OpenAI\Codex\bin\8e5b6932251c2c1c\codex.exe`.
- B: Polis T6 pinned Linux app-server under WSL+bwrap, `.tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex`, launched as `/codex app-server --stdio`.
- C: existing no-turn app-server preflight using the B runtime path; it reaches initialize/thread registration without sending a provider turn.

## First confirmed divergence

The first process-envelope divergence exists before provider contact:

- A: Windows executable SHA `e5aa76d1...`, Codex `0.153.4`, Windows desktop process ancestry, default user-profile home/config.
- B/C: Linux executable SHA `9739cbc9...`, Codex `0.151.0`, WSL+bwrap, `--clearenv`, explicit `/home/codex`, generated config, and `app-server --stdio`.

The first transport-relevant divergence is proxy injection:

- A observed shell: `POLIS_NATIVE_PROXY`, `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, and `NO_PROXY` are absent. Windows user/system proxy configuration is present, but the actual per-process network route cannot be proven without a live run.
- B/C: the controller uses `POLIS_NATIVE_PROXY`; bubblewrap does not inherit that name but explicitly injects localhost `HTTP_PROXY` and `HTTPS_PROXY` with digest `75e38c4b...`.

## Matrix

### confirmed_same

- Auth source file is the same host `C:\Users\chyinan\.codex\auth.json`, mounted read-only as `/home/codex/auth.json` for B/C. Opaque file fingerprint: `775b04c0...`.
- Model name in the visible A config and B request is `gpt-5.6-luna`.
- No visible OpenAI/Codex endpoint/base-URL override is present in A environment/config or B environment/template. An unrelated `ANTHROPIC_BASE_URL` child-shell setting exists in A config and is not classified as a Codex provider endpoint.
- Code-mode-host exists in both paths.

### confirmed_different

- Codex version: A `0.153.4`; B/C `0.151.0`.
- Executable path, binary SHA, platform, and runtime envelope differ.
- CWD: A current task `D:\Programs\Polis`; B/C `/work`.
- HOME/CODEX_HOME: A environment variables absent and defaults to the user profile; B/C explicitly `/home/codex`.
- Config: A uses `C:\Users\chyinan\.codex\config.toml` (SHA `95e8b796...`); B/C use a generated ephemeral `/home/codex/config.toml`. The exact historical T6 config file is no longer present, so its file SHA is not backfilled.
- Reasoning default: A config `max`; B request `medium`.
- App-server invocation: current A processes are app-server processes without `--stdio`; B/C use `app-server --stdio`.
- WebSocket configuration representation: B explicitly disables provider/response WebSockets; A has no equivalent explicit websocket keys in the observed config, so it uses application/default behavior.
- Proxy environment: A observed environment has no proxy variables; B/C inject localhost HTTP/HTTPS proxy values.
- B uses `--clearenv` and adds PATH, HOME, CODEX_HOME, LANG, HTTP_PROXY, and HTTPS_PROXY explicitly; these are not inherited from A's normal shell environment.

### not_recorded / cannot_determine_without_live_run

- Actual A per-process HTTP/SSE/WebSocket/fallback route.
- Whether A traffic uses the configured Windows proxy, VPN interception, or another desktop networking layer.
- B's exact response streaming fallback mode beyond the structured `responseStreamDisconnected` event.
- Historical B/T6 process tree and callback IPC endpoint.
- Exact historical T6 generated config file digest.
- A/B opaque stable account identity distinct from the auth file bytes.
- WorkerAdapter source fingerprint for T6; repository HEAD is known, but T1-T7 harness changes were uncommitted and no per-run source digest was recorded.

## Qualification-key finding

T7's L1 key records `auth_source_class` but not a stable opaque account/session identity. A path-preserving account change could therefore retain the same key. A future design may add a salted/HMAC opaque fingerprint over a stable non-secret account identifier or authenticated-session metadata. T8 does not modify the auth system or T7 record.

## Product boundary

L1 unqualified blocks paid/native Worker startup, business allowance reservation, and write-capability activation. It should not become a product-wide ban on persisting durable Mission/Task intent. Intent persistence and paid execution remain separate states; no harness refactor was performed in T8.

## Minimal next live canary

The recommended next experiment is one separately authorized, zero-tool, tiny-prompt canary using the exact T6/B combination while changing only proxy mode: remove the localhost `HTTP_PROXY`/`HTTPS_PROXY` injection. Keep binary `0.151.0`, WSL+bwrap, auth source, HOME/CODEX_HOME, model, effort, sandbox, prompt, and deadlines unchanged.

This is a single-variable transport test. It is not authorized or executed by T8. If a future test instead changes to Codex `0.153.4`, that must be a separate binary/version canary, not combined with the proxy change.

T7 remains unchanged:

```ini
native_execution_environment = unqualified
evidence_result = inconclusive
business_execution = blocked
```
