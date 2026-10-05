# Slice 246 — REQ-25 cross-tab owner authorization invalidation

## Change

Owner logout and protected 401/403 invalidation now clear protected views in other same-origin tabs. The current tab is invalidated immediately; other tabs receive an authorization-invalidation marker through `BroadcastChannel`, with a `storage` event fallback. The marker contains no cookie, CSRF token, or account data, and receiving it does not rebroadcast. If cross-tab messaging is unavailable, the current tab still clears its view.

This narrows stale-cache exposure after logout/revocation. It does not qualify browser, Tauri WebView, or remote-browser authentication behavior.

## Verification

- `npm run build` passed (`tsc -b`, Vite production build); the existing large-chunk advisory remains.
- `git diff --check HEAD^ HEAD` passed after integration.
- Traceability remains 17 open requirements; all 232 scenarios remain `not_run`.
- No tests, owner-session operation, DB operation, Worker/provider operation, browser/WebView session, or frozen scenario ran.
