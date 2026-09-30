# R0.3A-T21 — Windows-native full 11-tool L2 live canary

## Result

`T21 = NOT_STARTED / preflight_failed`. The offline V7 preflight passed, but
the Windows runner stopped at its own exact-surface guard before process start.
No allowance was created, no Medium was started, no provider egress occurred,
and no retry was performed.

## Offline V7 preflight

T20 remained the control with fingerprint
`b653e94bf2adfd4caa7f9982774f8d1143bdc55c0c64d50d620afdad3c8bfb7d`. T21
generated `canonical-manifest-v7` from the formal `codex.PeerBackendTools()`
registry and T5 reference. The exact surface contains 11 tools in this order:
`polis_work_current`, `polis_context_read`, `polis_workspace_read`,
`polis_workspace_replace`, `polis_contract_propose`, `polis_contract_accept`,
`polis_contract_read`, `polis_collab_send`, `polis_workspace_check`,
`polis_work_checkpoint`, `polis_artifact_submit`.

The T21 fingerprint is
`f3f4e0f8dc64ed373bea7716237ad14932cf98c887acead7e6b5d193216b90af`.
Aggregate schema bytes are `1792`; aggregate schema digest is
`f2fed5d1c86ce1cddc2c609bb31567fdced409b4eb12010f70b93456a72dc338`; the
formal tool manifest digest is
`b36063bf6b136b8116a1b744042e5f0228f46d559ba711e080e82b271e59645a`.
The normalized `thread/start` payload is `3591` bytes with digest
`8f76af5527da0b4197c40933074d9dcb48bf9140b0e5840e01a4a03ba7de7202`.

Auth recheck passed with identity
`dadcac24599324742b4afd9153211bcce573e0db8f6ed3ac88e065e175bc1b58` and
credential revision
`3076536bcf87f9c619736806c7bad3cc39a57e898bd0dcd8cde72e299c6ef6d7`.
Codex 0.153.4, Windows binary/code-mode-host, model, medium effort,
diagnostic HOME/config, native-default transport, no injected proxy, runtime,
deadlines and stop semantics matched the T20 control. Only the tool surface
and its legal derived manifest/capability/request fields differed.

## Failure evidence

The formal registry and all V7 evidence were present, but PowerShell loaded the
JSON array through a nested `@(...)` expression and observed one array object
instead of 11 elements. The runner therefore recorded
`T21 exact 11-tool surface is unavailable` before process start. This is a
runner preflight implementation failure, not evidence against Windows-native
provider compatibility. The saved run records `medium_started=0`,
`provider_egress=0`, `process_started=false`, `turn_started=false`, and no
attempted tools or business side effects.

The runner was not corrected and rerun because the authorization explicitly
forbade retry. No T21 provider result, L2 qualification, or real-Backend
eligibility was established. T20 and all earlier evidence/conclusions remain
unchanged.

Evidence: `evidence/development/r0.3a-t21/`.
