# T14 WebSocket-policy-only canary

- T14 did not reach process start or provider invocation. Its preflight manifest initially passed the intended policy diff but accidentally retained T10's explicit-disabled capability digest.
- The effective native-default capability digest is `33eda3b9b9fd48d26bc4989f8ae34c015eba8945efdf87e86aae3385a036443`; the attempted manifest held `105552b04a3e7379adf28dd266c1c5108fedbef824173a6455003af20647affb`. This is a harness construction error, not an observed transport divergence.
- T14 allowance exists with `medium_turns=0`, `high_turns=0`; no process, initialize, thread/start, turn/start, provider event, usage, or stop proof was produced.
- T14 status is `NOT_STARTED / preflight_failed`; no actual WebSocket/fallback transport can be inferred. The corrected future runner assigns the native-default capability digest before creating the v3 manifest, but no retry is authorized in this run.
