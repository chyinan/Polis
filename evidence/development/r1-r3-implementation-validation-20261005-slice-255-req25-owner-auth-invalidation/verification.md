# Slice 255 — REQ-25 owner page authorization invalidation

## Change

The installation owner page now sends protected provider-account reads and logout responses through the shared authorization observer. A 401/403 therefore clears the local protected view and notifies other same-origin tabs, including when logout itself is rejected. Login credential errors, owner bootstrap, and unauthenticated setup-status checks do not trigger this protected-data invalidation path.

This closes a local invalidation gap; supported browser/Tauri WebView behavior and remote-browser authorization remain unqualified.

## Verification

- `npm run build` passed in `frontend/`; the existing large-chunk advisory remains.
- `git diff --check` passed.
- All six REQ-25 scenario rows remain `partial/not_run`; all 232 frozen scenarios remain `not_run`.
- No tests, owner account/session flow, database operation, Worker/provider action, browser qualification, or frozen scenario ran.
