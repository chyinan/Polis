# R0.3A-T10 — Codex 0.153.4 Native Compatibility Qualification

Result: `PASSED` for local process/protocol/native-client compatibility.

```ini
codex_0_153_4_native_compatibility = PASSED
eligible_for_version_only_live_canary = YES
```

This is an offline qualification only. It did not reserve or consume Medium/High allowance, did not call a provider, did not run a model turn, and did not start Backend, Frontend, Reviewer, or T11.

## Binary and source

The Linux artifact is independent of the Windows 0.153.4 executable:

- source: official OpenAI npm platform package `@openai/codex@0.153.4-linux-x64`, configured registry mirror `https://registry.npmmirror.com/`;
- package target: `x86_64-unknown-linux-musl`;
- Linux binary: `.tools/codex-linux-0.153.4/package/vendor/x86_64-unknown-linux-musl/bin/codex`;
- version: `codex-cli 0.153.4`;
- binary SHA256: `56ef98ab4032d317ab26e9b5e5a175650717351edb16ed9cde0cb6d1734d62da`;
- code-mode-host SHA256: `3e85d67471825f73d02ff5f7e047ca1f6ca8caa3f59e4c6e8d9ca6ca7302cb45`;
- package tarball SHA256: `54818cb9fce3360cc6e44cfc5a96952cd5c1243efb43cbe488e11dda84663e08`.

The pinned 0.151.0 Linux binary remains at its original path and SHA256 `9739cbc928b9c573be83256acd46668f5dd4f119d2d09e05246895ca2aaf0c9a`. It was not replaced or rewritten; the old no-turn protocol test still passed against it after the compatibility change.

## Native protocol and schema

0.153.4 supports `app-server --stdio` and `app-server generate-json-schema`. The generated schemas are stored under `native-schema-0.153.4/`; the exact file-level comparison with the 0.151.0 generated schema is `schema-diff.json`.

The schema changed materially: 413 old JSON files versus 304 new files, 3 added, 112 removed, and 39 changed. The current Polis paths needed by this qualification remain present: `initialize`, `thread/start`, `thread/started`, `turn/start`, `turn/completed`, `item/tool/call`, and structured `error.responseStreamDisconnected`. The diff was recorded rather than inferred from a Windows binary or hidden behind a version-only assertion.

## Checks

Using the unchanged WSL+bwrap, `app-server --stdio`, isolated `/home/codex`, `/work`, no injected proxy, WS-disabled config and local no-provider path:

- process start and code-mode-host `--help`: passed;
- initialize/handshake: passed;
- local `thread/start` and `thread/started`: passed;
- dynamic-tool registration with the existing 7-tool surface: passed;
- stop and process termination proof: passed;
- native protocol parser: passed against the actual 0.153.4 trace;
- structured reconnect error classification: passed with a local 0.153.4-shaped `responseStreamDisconnected` event;
- T1/T2.1 reconnect, deadline, replay, stop-confirmation and terminal-error tests: passed.

The actual 0.153.4 no-provider trace is at `native-client-0.153.4/evidence/protocol.jsonl`. It contains no `turn/start`, provider response, usage update, or model output. Provider-dependent fields remain `not_run / compatibility_unproven`.

## Fingerprints and historical boundary

Future qualification records use:

```json
{
  "fingerprint_schema_version": "canonical-manifest-v1",
  "canonical_manifest_digest": "587ec5235f96fae24f688d49a8549e8251e394fa81d88f940cd04c2a8f8d1099"
}
```

The historical T7 key `58c8d49d9545d179820f03a4505163970bcaf461d6720fd3541640a5113cba45` is preserved as `legacy-json-serialization-v0`. It is not recomputed, overwritten, or silently treated as the new canonical digest.

T9 remains unchanged:

```ini
minimal_transport_canary = INCONCLUSIVE
no_proxy_L1 = unqualified_for_business_execution
```

T10 stops here. A future T11, if separately authorized, must be version-only and must not be started automatically by this qualification.
