# T13 current-auth version-only 0.153.4 canary

- T12 control manifest: `f50928d55cab833759f4796a3281b4b1803ebf82c248f6ec896dde8fb90a9516`.
- T13 canonical-manifest-v2 fingerprint: `f8db1f97ba8d5fe37c01f0442a50273f742f414e2b718926dba1f401dddd7cd0`.
- Auth remained exact: source `mounted_codex_auth_file`; identity `dadcac24599324742b4afd9153211bcce573e0db8f6ed3ac88e065e175bc1b58`; credential revision `3076536bcf87f9c619736806c7bad3cc39a57e898bd0dcd8cde72e299c6ef6d7`.
- T12↔T13 factor diff passed with only version and derived binary/code-mode-host/capability/native protocol fields changed; runtime, config/invocation, proxy, WebSocket-disabled policy, model/effort, tools, prompt, deadlines and auth were unchanged.
- One `gpt-5.6-luna / medium` turn was reserved and started; no High/retry/reset. Raw protocol shows initialize, thread/start, thread/started, turn/start, turn/started, user-message item, two structured `responseStreamDisconnected` events in `pre_first_output_reconnecting`, no valid output, no usage updates and no `turn/completed`.
- Terminal state: `first_valid_output_deadline_exceeded`; stop proof: `process-group:435:waited`; T13 is `INCONCLUSIVE`, Case B, and L1 is `unqualified_for_business_execution`.
- T13 does not establish current-environment version causality because 0.153.4 reproduced the current-auth 0.151.0 failure. The next candidate is a separately authorized WebSocket-policy diagnostic; no T14/L2/Backend starts automatically.
