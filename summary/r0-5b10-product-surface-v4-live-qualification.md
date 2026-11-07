# R0.5B10 product tool surface v4 live qualification

## Current state

- Exact production surface: `polis-product-tool-surface@4`, 7 tools.
- Manifest: `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`.
- Aggregate schema: 2206 bytes, digest `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`.
- B9 surface identity: exact match; post-canary surface freshness: exact match.
- Offline L2: PASSED, including B9 deterministic delivery and frozen LIVE_2 counterfactual regressions.

## Live boundary

The one diagnostic authorization was `PRODUCT_SURFACE_V4_LIVE_QUALIFICATION`, `gpt-5.6-luna/medium`, reservation/attempt/egress limits `1/1/1`, High/retry/business allowance `0/0/0`. The generated exact-surface execution fingerprint was `d931a5faf01f72f23ad91fd16f812a5e2b240b967f57b7fe9ab1b718db76bf61`.

The single attempt is terminal `INCONCLUSIVE`: process start and initialize response occurred, but initialize failed closed because authorization expected `0.155.0-alpha.9.2` while the provider userAgent reported `0.154.0-alpha.6.2`. Provider egress, thread/start, turn/start, first output, tokens, tool calls, and reconnects were all zero. Clean stop and duplicate reservation denial passed. No provider L2 fingerprint was created, and @4 is not qualified for future business samples.

## Evidence and verification

Primary evidence: `evidence/development/r0.5b10-product-tool-surface-v4-live-qualification/qualification-report.md`.

Post-canary targeted/full Go tests, race, vet, Linux/Windows command builds, frontend npm test/typecheck/lint/build, disposable PostgreSQL/CAS regressions, Bash/Python syntax and `git diff --check` passed. The first B10 runner build retained a historical B8 runtime-root default; source was corrected after the terminal attempt, without any provider retry.
