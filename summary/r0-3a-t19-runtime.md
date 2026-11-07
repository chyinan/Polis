# R0.3A-T19 — Windows-native vs WSL/Linux Runtime Differential Qualification

## Result

`T19 offline qualification = PASSED`.
`eligible_for_windows_native_minimal_live_canary = YES`.
No Medium allowance was created, no provider/model egress occurred, and no live
comparison or business execution was started.

## Windows diagnostic runner

The Windows runner used a temporary external diagnostic `CODEX_HOME`, selected
only the existing T14C non-secret `config.toml`, and used a temporary auth
snapshot of the controlled auth material. The daily
`C:\Users\chyinan\.codex` profile was not used or copied; history, cache,
sessions, plugins, MCP definitions and unrelated state were excluded.

Windows Codex `0.153.4` successfully completed process start, initialize,
`app-server --stdio`, `thread/start`, stdio framing and parser-facing local
protocol handling, then stopped with a confirmed Windows Process handle wait.
No `turn/start` was sent. Input framing was CRLF; output framing was LF;
redirected standard pipes were used. `codex-code-mode-host.exe` resolved from
the sibling installation path. No Windows Job Object was used; the lifecycle
proof was direct process-handle wait. Structured-error parser coverage remains
from offline Go fixtures, not a provider call.

## Normalized differential

V6 fingerprints:

- Windows diagnostic: `b653e94bf2adfd4caa7f9982774f8d1143bdc55c0c64d50d620afdad3c8bfb7d`
- WSL/Linux+bwrap: `bba78182f9347618b1d6b80f39a114fe0f813a2af96aaae5dddc68beaeba0653`

Controlled-same factors were confirmed for platform-independent semantics:

- Codex version `0.153.4`
- `app-server --stdio`
- model `gpt-5.6-luna`, effort `medium`
- selected non-secret config and transport-config digests
- controlled auth source semantics, identity and credential revision
- no injected proxy and `native_default` provider transport policy
- zero dynamic tools and read-only sandbox semantics
- diagnostic workspace role
- identical prompt/developer digests
- identical local handshake protocol subset and parser compatibility

Runtime-derived differences were limited to Windows/Linux binary and
code-mode-host SHA, diagnostic HOME profile, native OS vs bwrap filesystem and
process isolation, `CreateProcess` vs bwrap launch, Windows redirected pipes vs
Go pipes, CRLF input vs LF input, Windows native network vs WSL shared network,
and resulting capability digest.

There were no `unexpectedly-different` or `not-comparable` factors in the
controlled comparison. Full native schema digest parity was not claimed:
Windows full schema digest is `not-recorded`; the local handshake subset was
explicitly measured and matched.

Auth identity/revision remained comparable. Config comparability is limited to
the selected diagnostic config, not the full Windows user profile. Configured
transport policy is comparable; actual provider transport was not observed.

## Next authorization

The future next step is exactly one separately authorized Windows-native
minimal live canary: one Luna Medium, zero tools, tiny prompt, no business side
effects, same controlled auth, no retry/reset. T19 itself does not create that
allowance. T18/T14C/T16/T17 and all historical evidence remain unchanged.

Evidence: `evidence/development/r0.3a-t19/`.
