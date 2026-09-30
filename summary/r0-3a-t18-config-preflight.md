# R0.3A-T18 — Selected CODEX_HOME Config-loading-only Preflight

## Result

`T18 = NOT_STARTED`. The offline preflight found no safe, concrete single
non-secret config key that could be changed while preserving the registered
T14C execution factors. No Medium, High, provider egress, allowance, or live
canary was used.

## Assessment

`selected_config_keys = []`. The before/after values intentionally remain equal
because no candidate was materialized:

- effective config digest: before/after
  `a342c76493b2de2c24f4001f81206678e5b70f6b78a2427b3c40ac6ccc7caf7c`
- transport config digest: before/after
  `e5e78e023210da320fdf86f3c8504a3c97d535d11fb0a732d6e36229f2a57eb3`

Omitted fields were classified as fixed/prohibited (`medium`, `read-only`,
shared network, zero tools, plugins/MCP disabled), provider-visible
(`model_provider`), credential-controlled, sensitive endpoint/shell-env, or
unproven cross-runtime default resolution (WebSocket/native-default keys).
History, cache, session DBs and unrelated state were excluded.

## Auth boundary

The isolated host HOME's zero-byte `auth.json` placeholder is explicitly not a
credential source. The actual credential source is the controlled auth file
mounted read-only at guest `/home/codex/auth.json`. Only its
`mounted_codex_auth_file`, identity fingerprint and credential-revision
fingerprint are carried into the diagnostic proof; no raw credential material
was written.

## Decision

`same_native_default = UNPROVEN` remains unchanged. Because no valid selected
config-only execution combination exists, no T18 live canary is legal. The one
next step is a separately authorized Windows-native vs WSL/Linux runtime
differential; do not continue splitting config keys. T14C and all earlier
historical evidence remain unchanged.

Evidence: `evidence/development/r0.3a-t18/`.
