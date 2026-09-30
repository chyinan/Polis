# T11 transport harness investigation

## Verified entry points

- `cmd/polis-r03a-t9/main.go` hardcodes the historical 0.151.0 binary, runtime root, and T9 evidence path.
- `internal/probe/r03a_t9.go` implements T9 proxy-removal preflight and delegates the live canary to `RunR03AT6`.
- `internal/probe/r03a_t6.go` owns the one-turn canary lifecycle: persisted allowance, readiness manifest, native process, initialize, zero-tool thread/start, one Medium reservation, phase-aware `TurnWithOptions`, protocol analysis, result and stop proof.
- `internal/codex/client.go` provides `NewWithModelAndVersion`, so a version-specific client can validate 0.153.4 without changing the historical default.
- `internal/runner/native.go` provides the unchanged WSL+bwrap `app-server --stdio` envelope and derives the capability digest from the config, proxy string and code-mode-host bytes. Its historical version constants remain 0.151.0.
- `internal/codex/transport_turn.go` and `internal/probe/canary_protocol.go` already implement the registered reconnect phases, deadlines, usage counting and first-output extraction needed by T11.

## Frozen T9 baseline

- Historical no-proxy T9 legacy execution key: `ca04fb2fe5004a7c14b8ccbae7bc7b043d010ad517c571c1c04136415c20b782`.
- T9 runtime/model/effort/sandbox/cwd/proxy/tool/prompt state is recorded in `evidence/development/r0.3a-t9/luna-1/`.
- T9 live outcome: `turn/started`, user-message item, three pre-first-output structured reconnects, no output/usage/turn-completed, first-output deadline, confirmed process stop.

## T10 version evidence

- Independent Linux 0.153.4 binary SHA-256: `56ef98ab4032d317ab26e9b5e5a175650717351edb16ed9cde0cb6d1734d62da`.
- Its code-mode-host SHA-256 is `3e85d67471825f73d02ff5f7e047ca1f6ca8caa3f59e4c6e8d9ca6ca7302cb45`.
- T10 canonical qualification digest is `587ec5235f96fae24f688d49a8549e8251e394fa81d88f940cd04c2a8f8d1099`; its native schema/protocol digest is `d052a9f423be59cd61cffb9dd8bf65b4e1a161b49800706c136c2f8862976244`.
- T10 explicitly says provider egress/live model turns/business execution were not run and authorizes a separately gated version-only canary.

## T11 implementation boundary

Add a T11-specific functional preflight that compares the frozen T9 combination to a 0.153.4 combination and rejects every field change except `codex_version`, `binary_sha256`, `code_mode_host_sha256`, `capability_digest`, and `native_protocol_digest` as version-derived fields. Add a thin imperative runner using the existing native client and deadlines, writing only under a new `evidence/development/r0.3a-t11/luna-1/` directory. Do not alter T9 code or evidence, and do not expose a retry path.
