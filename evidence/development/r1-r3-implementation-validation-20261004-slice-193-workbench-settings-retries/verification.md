# Slice 193 verification — Workbench guidance and settings retries

Date: 2026-10-04

Operator guidance, runtime settings, Company updates, and Company archive commands now preserve their request ID for an exact same-payload retry after an ambiguous response. The identity is cleared only after success; changing the payload uses a different request ID. Their existing settle handlers refresh the authoritative Workbench projections.

Verification performed:

- `cd frontend && npm run build` — passed (TypeScript project build and Vite production build). Vite emitted its existing advisory that the generated JavaScript chunk exceeds 500 kB.
- `git diff --check` — passed.

No tests were run. No guidance or settings command was submitted, no Company was archived, and no Worker/provider action or frozen scenario ran. No schema migration occurred. Runtime restart and external credential qualification remain open.
