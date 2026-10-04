# Slice 185 verification — R3 domain evidence command reconciliation

Date: 2026-10-04

Content and research evidence mutations now invalidate the persisted `domain-evidence` Workbench projection on settle, whether a command succeeds or errors. Covered mutations include source authorization, draft registration, review, simulated publication, correction and feedback, research simulation, domain evidence submission/review, substantive assessment, and profile qualification evidence.

Verification performed:

- `cd frontend && npm run build` — passed (TypeScript project build and Vite production build). Vite emitted its existing advisory that the generated JavaScript chunk exceeds 500 kB.
- `git diff --check` — passed.

No tests were run. No source authorization, content or research evidence, or profile qualification was recorded. No research simulation, simulated publication, external publication, Worker/provider action, or frozen scenario ran. No schema migration occurred. The R3 profiles remain `not_run` and execution-disabled pending independent evidence.
