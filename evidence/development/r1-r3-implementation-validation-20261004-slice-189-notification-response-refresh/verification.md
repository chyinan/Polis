# Slice 189 verification — notification command response recovery

Date: 2026-10-04

The local notification route and test commands now refresh notification projections after either outcome. Disabled QQ notification-draft writes also refresh their projection. Workbench controls preserve the exact request ID for the same route, draft, or test intent and tell the operator to inspect refreshed state when a response is ambiguous.

Verification performed:

- `cd frontend && npm run build` — passed (TypeScript project build and Vite production build). Vite emitted its existing advisory that the generated JavaScript chunk exceeds 500 kB.
- `git diff --check` — passed.

No tests were run. No notification route/draft changed, test notification was sent, QQ credential was used, external API was called, Worker/provider was started, or frozen scenario was executed. No schema migration ran. Real QQ notification qualification remains `not_run`.
