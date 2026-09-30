# Slice 78 verification — R2 remote Workbench access profile

Date: 2026-09-29

## Implemented boundary

- Remote access is default-off and reuses one operator-configured same-origin HTTPS reverse proxy. `POLIS_REMOTE_WORKBENCH_ORIGIN` must be exact/canonical, HTTPS, and paired with `POLIS_DESKTOP_SESSION_TOKEN`.
- Remote profile startup requires the Polis HTTP server to remain loopback-bound. The application does not open a public listener, TLS socket, or tunnel.
- The desktop middleware permits only the configured remote Origin, still requires the session token, and rejects query-string token auth for remote-Origin and no-Origin requests. The existing query fallback remains only for explicitly allowed local desktop Origins to support local EventSource.
- The reverse proxy must perform user authentication/TLS, strip user-supplied session-token headers and `desktop_token` query values, and inject the protected service token after authentication.
- The Workbench labels access `local_management` only when both page and API origins are loopback; if either origin is remote or invalid, it shows `remote_management_unqualified`. It never claims `remote_management_qualified` from configuration alone.

## Verification

- RED: `rtk bash scripts/go.sh test ./internal/desktop -count=1` failed on missing/invalid remote-origin configuration and remote-origin middleware behavior. Review follow-up added regression cases; the no-Origin query-token case failed with 204 before the fix. Frontend review follow-up added a local-page/remote-API case, which failed before origin-pair classification.
- GREEN: the targeted desktop package passed after adding exact HTTPS validation, loopback requirement, origin allowlisting, token enforcement, query-token rejection for remote and no-Origin requests, and local EventSource compatibility.
- `rtk bash scripts/go.sh test ./... -count=1` — passed across all Go packages after both review fixes.
- Frontend Vitest — 132 tests passed across 14 files, including local page/API pairs, remote page/API origins, and local versus remote-unqualified mode presentation. Typecheck and lint passed.
- `npm run build` — passed. Vite emitted the existing advisory that the main minified chunk exceeds 500 kB (713.11 kB).
- Linux and Windows amd64 command builds — passed.
- `scripts/test-migration-hash-manifest.ps1` and `rtk git diff --check` — passed.

The first review found two issues: remote mode accepted a query token without an Origin header, and the UI ignored an absolute remote API base URL when the page was local. Both are covered by regression tests and fixed; read-only re-review found no new remote-authentication bypass or qualification mislabel. No external reverse proxy, certificate, network listener, account, tunnel, or mobile browser was used. The actual authentication-provider behavior, TLS deployment, notification-to-login flow, takeover action, resulting remote state, and `remote_management_qualified` evidence remain not_run. The exact deployment profile and operator contract are documented in `docs/implementation/R2_REMOTE_WORKBENCH_ACCESS.md`.
