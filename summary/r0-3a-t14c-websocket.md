# R0.3A-T14C — Corrected WebSocket-policy-only Minimal Transport Canary

## Result

`T14C = INCONCLUSIVE` (Case B). The native-default combination remains
`unqualified_for_business_execution` at L1. No retry, reset, T15, L2, Backend,
Frontend, successor, Reviewer, or other model call was started.

## Preflight

T14A's corrected `T14AEffectiveExecutionConfig` was the single source for the
launch args/environment, native config bytes, capability, launch digest,
canonical-manifest-v3 and qualification fingerprint. The T13 v2 control was
`f8db1f97ba8d5fe37c01f0442a50273f742f414e2b718926dba1f401dddd7cd0`; its v3
cross-schema expression was `ea49b5df52d7923a520cd80f053ba3f6a055698121e3985cdd51eb58b2d9cbd3`.
The T14C native-default candidate was `14ffcaa033f927e95d2f729b97495529e7b7b9f2d4a58ba8edda34c81c08fb75`.

The only confirmed factor diff was the transport policy plus its derived
capability digest:

- control: `explicitly_disabled`, capability `105552b04a3e7379adf28dd266c1c5108fedbef824173a6455003af20647affb`
- candidate: `native_default`, capability `33eda3b9bb9fd48d26bc4989f8ae34c015eba8945efdf87e86aae3385a036443`

Auth source, identity and credential revision were `SAME`; identity was
`dadcac24599324742b4afd9153211bcce573e0db8f6ed3ac88e065e175bc1b58`, and the
credential revision was
`3076536bcf87f9c619736806c7bad3cc39a57e898bd0dcd8cde72e299c6ef6d7`. No frozen
auth snapshot was used. The host source
`/mnt/d/Programs/Polis/.runtime/linux/r03a-t14c/home` was verified separately
from guest `/home/codex`; all bind sources existed, were readable, and matched
their expected type. T13's observed guest `/home/codex` and `/work` semantics
were preserved.

## Live evidence

One Luna Medium was started (`medium=1`, `high=0`, `provider_egress=1`). The
process, initialize, thread/start, turn/start, and user-message item were
observed. The provider emitted two structured response-stream disconnects in
the pre-first-output phase. There was no first valid output, recovery, native
usage update, token usage, or `turn/completed`. The first-output deadline was
`90,015 ms`; stop proof was `process-group:438:waited`. The configured policy was
`native_default`; the actual transport was `not_observable`, so this does not
prove that WebSocket was used.

Evidence: `evidence/development/r0.3a-t14c/luna-1/`. Historical T13, T14 and
T14B evidence was not rewritten.

## Interpretation

The corrected single-factor preflight was valid, but native-default did not
recover the current frozen execution combination. Therefore no strong recovery
causality claim is supported and any next transport-variable experiment needs
separate authorization.
