# R0.3A-T21A — Windows tool registry materialization qualification

## Result

`T21A = PASSED`; `eligible_for_new_T21B_live_canary = YES`.
This stage used zero Medium, zero High and zero provider egress. T21 remains
permanently `NOT_STARTED / preflight_failed`; its evidence and conclusion were
not changed. No T21B or Backend execution was started.

## Fix and regression

The T21 runner had used a nested PowerShell array expression around
`ConvertFrom-Json`. PowerShell consequently exposed the root JSON array as one
`System.Object[]` object, and the exact-11 guard stopped before process start.
T21A adds a narrow helper that explicitly requires a root array, iterates each
root element into a flat record list, rejects nested arrays, and validates the
formal registry against the Go-generated surface metadata.

The local regression passes for the real T21 registry and rejects nested arrays,
wrong count, missing tools, reordered tools, per-tool schema digest mismatch,
aggregate manifest digest mismatch and missing callback/policy binding metadata.
Tool definitions continue to come only from `codex.PeerBackendTools()`.

## Corrected provider-before preflight

The fixed Windows runner was invoked once in `-PreflightOnly` mode against the
frozen T21 input evidence and a fresh T21A output directory. It completed local
process start, initialize and thread/start, then stopped before `turn/start`.
It observed `registered_tool_count=11`, `materialization_validation=passed`,
`binding_metadata_count=11`, formal tool manifest digest
`b36063bf6b136b8116a1b744042e5f0228f46d559ba711e080e82b271e59645a`, aggregate
schema digest
`f2fed5d1c86ce1cddc2c609bb31567fdced409b4eb12010f70b93456a72dc338`, and
aggregate schema bytes `1792`. Process termination was confirmed with stdin
close plus Windows Process handle WaitForExit, exit code `0`.

No live provider turn was made; there is no T21B result yet. The next permitted
step is a new separately authorized T21B exact-11-tool Windows live canary.
