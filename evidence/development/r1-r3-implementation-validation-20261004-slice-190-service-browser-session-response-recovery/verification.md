# Slice 190 verification — service browser session retry recovery

Date: 2026-10-04

Workbench now retains a pending service-browser request ID for the same JobRun when the command response is ambiguous. The ingress stores request-ID-to-session mappings in memory and returns the same session while it remains unexpired; an exact retry can therefore recover a committed response without creating another ticket. The existing five-minute expiry and stop-time revocation behavior are unchanged.

Verification performed:

- Source review of `internal/control/service_browser_ingress.go` confirmed same-request replay of an unexpired session.
- `cd frontend && npm run build` — passed (TypeScript project build and Vite production build). Vite emitted its existing advisory that the generated JavaScript chunk exceeds 500 kB.
- `git diff --check` — passed.

No tests were run. No JobRun or browser session was created; no Worker/provider or host action ran. No frozen scenario or schema migration occurred. Native browser/host qualification remains open.
