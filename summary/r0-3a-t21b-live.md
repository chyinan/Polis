# R0.3A-T21B — Windows-native exact-11-tool L2 live canary

## Result

`T21B = PASSED`; `windows_native_L2_11_tool_surface = QUALIFIED` and
`eligible_for_real_backend = true` under the authorized lifecycle criteria.
Exactly one `gpt-5.6-luna / medium` was started with one provider egress. No
retry, reset, second model, Backend, Frontend, successor, Reviewer or business
follow-on execution was started.

## Frozen factors

T20 control fingerprint:
`b653e94bf2adfd4caa7f9982774f8d1143bdc55c0c64d50d620afdad3c8bfb7d`.
T21B V7 fingerprint:
`f3f4e0f8dc64ed373bea7716237ad14932cf98c887acead7e6b5d193216b90af`.
The formal T21A-qualified tool manifest digest is
`b36063bf6b136b8116a1b744042e5f0228f46d559ba711e080e82b271e59645a`, with
11 registered tools and aggregate schema bytes `1792`.

Auth recheck passed with identity
`dadcac24599324742b4afd9153211bcce573e0db8f6ed3ac88e065e175bc1b58` and
credential revision
`3076536bcf87f9c619736806c7bad3cc39a57e898bd0dcd8cde72e299c6ef6d7`. Codex
0.153.4, Windows-native runtime, diagnostic HOME/config, native-default
transport, no injected proxy, medium effort and read-only policy stayed fixed.

## Live evidence

The process completed initialize, thread/start, turn/start, `turn/started`, the
user-message item, structured agent-message delta output and `turn/completed`.
First valid output arrived in `7421 ms`; reconnect count was `0`; native usage
updates were `1`; usage reported `inputTokens=11083`, `outputTokens=192`,
`reasoningOutputTokens=179`, and `totalTokens=11275`. No tool call was
attempted, business side effects were `0`, unresolved transport state was
`false`, terminal state was `completed`, and process stop was confirmed with
`stdin close plus Windows Process handle WaitForExit`, exit code `0`.

The actual delta output was `__TRANSPORT_SENTINEL__`, not the expected
`POLIS_TRANSPORT_CANARY_OK`; the derived result records
`sentinel_match=mismatch` and `sentinel_instruction_following=failed` without
changing the lifecycle transport PASS. The raw allowance records the correct
T21B qualification but has an empty `t21_fingerprint`; this metadata defect is
preserved and surfaced as `allowance_fingerprint_binding=mismatch`.

## Interpretation

This verifies Windows-native exact-11-tool provider compatibility and clean
diagnostic side-effect isolation for the current execution envelope. T5's
historical `11_tool_provider_causality = UNPROVEN` conclusion is not rewritten;
the new result is runtime/profile-specific evidence. T20, T21, T21A and all
earlier evidence remain unchanged. Transport/tool infrastructure is now frozen;
no Backend was started automatically.

Evidence: `evidence/development/r0.3a-t21b/`.
