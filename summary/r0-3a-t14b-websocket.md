# T14B WebSocket-policy-only canary attempt

- T14B used T13 auth identity/revision and the T14A native-default policy candidate, but did not reach the minimal transport turn. It stopped before Medium reservation with `medium_turns=0`, `high_turns=0`, and no provider model turn.
- Raw native evidence records only an initialize request followed by bwrap stderr: `Can't find source path /home/codex`. This was a launch configuration error: the shared T14A config conflated the guest HOME `/home/codex` with the host-side bind source path.
- The original preflight/manifest/result/session evidence is preserved as `*-attempt-1`; the main T14B evidence is explicitly `NOT_STARTED / preflight_failed`. No WebSocket or fallback transport conclusion is made.
- T14A was corrected offline to separate `HostHomePath` from guest `Home`; the corrected runner now derives and writes the same launch artifact used by the future process. A fresh T14B authorization and allowance are required; this T14B attempt is not retried.
