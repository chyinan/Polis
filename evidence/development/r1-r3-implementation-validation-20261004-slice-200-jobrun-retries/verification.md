# Slice 200 verification — JobRun and handover exact retries

Date: 2026-10-04

Cross-backend handover creation, isolated batch/service JobRun start, and JobRun stop retain exact request IDs for same-payload retries after ambiguous responses. Retry keys cover source/target and Task or JobRun scope plus environment/session and script/argument or service configuration. Successful responses clear the pending identity.

Verification performed:

- `cd frontend && npm run build` — passed (TypeScript project build and Vite production build). Vite emitted its existing advisory that the generated JavaScript chunk exceeds 500 kB.
- `git diff --check` — passed.

No tests were run. No handover or JobRun was created, started, or stopped. No Worker/provider action or frozen scenario ran. No schema migration occurred. Actual host execution qualification remains open.
