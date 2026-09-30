# R0.3A-T20 — Windows-native minimal live transport canary

## Result

`T20 = PASSED`; `windows_native_L1 = qualified` for the frozen Windows
diagnostic profile. Exactly one Luna Medium was started and one provider egress
was recorded. No retry, reset, second model, WSL live comparison, L2, Backend,
Frontend, successor, Reviewer or business execution was started.

## V6 preflight

The new T20 manifest reused the T19-qualified Windows V6 execution factors and
rechecked the current binary, code-mode-host, selected non-secret config and
opaque auth material before the process launch. The fingerprint is
`b653e94bf2adfd4caa7f9982774f8d1143bdc55c0c64d50d620afdad3c8bfb7d`, equal to
the T19 Windows factor fingerprint because the live run does not itself change
the execution combination. The execution-config digest is
`70711c3894a1fd8c6bc8c3d0284c3f164915b9fd50be44955cc32795a470a169`; the
launch-config digest is
`711cb622329540cbf996902cfeaae8b13a54976318bdd0a423b684ca7bdba0ea`.

Auth source semantics were `controlled_diagnostic_auth_material`; identity was
`dadcac24599324742b4afd9153211bcce573e0db8f6ed3ac88e065e175bc1b58`; credential
revision was
`3076536bcf87f9c619736806c7bad3cc39a57e898bd0dcd8cde72e299c6ef6d7`. The
actual auth bytes were copied only to an external temporary read-only
diagnostic snapshot; no credential contents were written to evidence.

## Live lifecycle

The Windows-native `0.153.4` `app-server --stdio` process completed initialize,
thread/start, turn/start, turn/started, the user-message item, agent output and
turn/completed. First valid output arrived in `4800 ms`; reconnect count was
`0`; native usage updates were `1`; usage reported `inputTokens=10523`,
`cachedInputTokens=4864`, `outputTokens=11`, `reasoningOutputTokens=0`, and
`totalTokens=10534`. Terminal state was `completed`; no unresolved transport
state remained. Process termination was confirmed with `stdin close plus
Windows Process handle WaitForExit`, exit code `0`.

The runner's aggregate text included the delta output and repeated the later
completed-item text, so its raw summary records `sentinel_match=mismatch` and
is preserved. `transport-summary.json` derives the output from only the saved
agent-message delta events and records the exact sentinel
`POLIS_TRANSPORT_CANARY_OK`; this is a derived parser correction and did not
rerun or alter the provider turn.

## Interpretation

The current result is `runtime_execution_envelope_factor =
STRONG_DIAGNOSTIC_EVIDENCE`: the Windows-native diagnostic envelope completed
while frozen WSL/Linux historical runs T12/T13/T14C were no-output reconnects.
This does not identify WSL itself as the root cause; the runtime envelope also
contains OS, process, filesystem, stdio and platform-derived differences.
T6/T9/T12/T13/T14C and all historical evidence remain unchanged.

Evidence: `evidence/development/r0.3a-t20/`.
