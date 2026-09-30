# R2 remote Workbench access profile

## Fixed boundary

Remote browser access reuses an operator-configured, same-origin HTTPS reverse proxy. Polis continues to bind its API to loopback; it does not create a listener, public port, tunnel, certificate, or user login. The reverse proxy owns user authentication and TLS, and injects the protected Polis session token only after authentication.

The operator sets:

- `POLIS_WORKBENCH_ADDR=127.0.0.1:8080` (or another loopback address)
- `POLIS_DESKTOP_SESSION_TOKEN` from protected deployment storage
- `POLIS_REMOTE_WORKBENCH_ORIGIN=https://<exact-host>[:port]`

Startup rejects a remote origin without a token, an HTTP origin, a path/query/fragment, wildcard or user-info origins, and a non-loopback Polis listener. The middleware allows that one exact HTTPS browser origin and still requires the session token. With the remote profile enabled, query-token authentication is disabled for remote-Origin and no-Origin requests; it remains available only for explicitly allowed local desktop Origins so local EventSource can work.

The reverse proxy must require its configured user authentication, remove client-supplied `X-Polis-Desktop-Token` and every `desktop_token` query value, then inject the protected service token. It must authenticate SSE requests as well as ordinary API requests. Do not log the injected header or token. Keep the Polis listener on loopback; do not forward directly from an untrusted network.

## Product status and qualification

The Workbench labels local page and API origins `local_management`; if either origin is non-loopback or invalid, it shows `remote_management_unqualified`. Configuration does not set `remote_management_qualified`. The fixed status is intentionally honest until an owner-approved mobile test validates notification discovery, login, risk inspection, a takeover action, and the resulting authoritative state through the actual TLS/auth/network deployment. This code path does not add a URL or bearer token to QQ notifications.

## Local verification boundary

Unit tests cover canonical HTTPS origin validation, required token, loopback-only binding, exact-origin allowlisting, token rejection, and query-token rejection for remote-Origin and no-Origin requests. Frontend tests cover local page/API pairs and remote page or API origins. A local reverse-proxy/mobile end-to-end qualification has not run; no public listener, tunnel, certificate, account, or remote connection was created.
