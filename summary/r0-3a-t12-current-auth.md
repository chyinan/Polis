# T12 current-auth 0.151.0 baseline

- T12 used the current external auth file directly; no diagnostic snapshot was created. Preflight recorded and the live path rechecked `auth-identity-v1` and `auth-credential-revision-v1` before the only Medium reservation.
- T12 canonical-manifest-v2 digest: `f50928d55cab833759f4796a3281b4b1803ebf82c248f6ec896dde8fb90a9516`.
- Auth source: `mounted_codex_auth_file`; identity: `dadcac24599324742b4afd9153211bcce573e0db8f6ed3ac88e065e175bc1b58`; credential revision: `3076536bcf87f9c619736806c7bad3cc39a57e898bd0dcd8cde72e299c6ef6d7`.
- Runtime remained 0.151.0, historical binary SHA `9739cbc928b9c573be83256acd46668f5dd4f119d2d09e05246895ca2aaf0c9a`, WSL+bwrap, `/work`, `/home/codex`, `app-server --stdio`, read-only, no proxy, WebSocket disabled and zero dynamic tools.
- One `gpt-5.6-luna / medium` turn was reserved and started. Raw protocol shows initialize, thread/start, thread/started, turn/start, turn/started, user-message item, two structured `responseStreamDisconnected` events in `pre_first_output_reconnecting`, no first valid output, no native usage update and no `turn/completed`.
- Terminal state: `first_valid_output_deadline_exceeded`; stop proof: `process-group:423:waited`; Case B. T12 L1 remains `unqualified_for_business_execution`.
- `transport-trace.json` and related derived summaries were repaired offline after a stderr-record parser defect; the raw native protocol was not changed and no second model call occurred.
- T12 is not a version comparison with T9 because the current auth credential revision differs from T9. A future T13 requires a fresh authorization and the exact same current auth identity/revision as T12.
