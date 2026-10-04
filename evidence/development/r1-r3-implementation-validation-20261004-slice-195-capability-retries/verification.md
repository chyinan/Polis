# Slice 195 verification — capability governance retries

Date: 2026-10-04

Capability package imports, metadata qualification, approval/revocation, runtime qualification/observation, employee binding and binding revocation preserve exact request IDs after ambiguous responses. For uploaded packages, the pending identity includes a SHA-256 digest of the selected bytes, revision, and target server where applicable.

Verification performed:

- `cd frontend && npm run build` — passed (TypeScript project build and Vite production build). Vite emitted its existing advisory that the generated JavaScript chunk exceeds 500 kB.
- `git diff --check` — passed.

No tests were run. No capability command, stdio observation, or Streamable HTTP endpoint observation was executed. No Worker/provider action or frozen scenario ran. No schema migration occurred. Real capability execution and host/account qualification remain open.
