# Slice 75 verification — deterministic R3 research simulator

Date: 2026-09-29
Schema: 53 (no migration)

## Implemented

- Added `RunResearchSimulation`, a pure deterministic implementation of the fixed `bootstrap-mean-difference@1` method. It accepts strict JSON dataset/method inputs only, checks exact same-company authorized MissionInput references and CAS digests, pins the protocol seed/control definition, and rejects unknown fields, unsupported methods/risk units, malformed or oversized data, and budget excess.
- The simulation uses a fixed SplitMix64 PRNG and emits canonical JSON with the input digests, protocol revision, seed, consumed sample-draw units and descriptive summary statistics. Repeating the same source bytes, method and seed produces identical output bytes and digest.
- It does not execute supplied code, access external services, classify positive/negative outcomes, persist a run, qualify the reference profile, or enable domain execution. Both R3 profiles remain `not_run` and disabled.

## Verification

- `rtk bash scripts/go.sh test ./internal/domainworkflow -count=1` — passed. Tests cover deterministic output, exact source digest and company binding, over-budget refusal, changed input bytes and unsupported risk units.
- This slice reuses the Schema53 full Go/frontend/PostgreSQL and Linux/Windows build verification recorded in the adjacent Slice74 evidence. The simulator itself makes no network calls.

## Remaining R3 work

Persist protocol and simulation-run records and surface them in Workbench. Implement the separate content draft, independent fact-check, human-sample, simulated-publication, correction and feedback lifecycle. Real domain review, quality, recovery, cost and organization-benefit evidence remain separate qualification gates.
