# Slice 69 — Service browser ingress verification

Date: 2026-09-29

## Implemented

- A ready service JobRun can mint a five-minute, one-time browser ticket only through the company/job-scoped Workbench command. The desktop middleware requires its session token even in tokenless loopback mode.
- The gateway binds to `127.0.0.1`, pins the immutable service probe address and port, and checks listener and accepted-connection ownership. A fixed bootstrap page sets an ingress-specific HttpOnly/SameSite=Strict cookie, then begins a same-origin navigation; the bearer ticket is removed from the final URL.
- Requests require a loopback peer and exact Host. Fetch Metadata blocks both cross-site and same-site cross-origin browser requests; unsafe methods also require the exact gateway Origin. The gateway rejects protocol upgrades, unpinned redirects, oversized requests/responses/headers, and duration/concurrency overflow. Response bodies are buffered up to the bound before response headers are committed, so chunked over-limit responses return 502 instead of a successful truncated page. The ingress cookie is removed upstream while ordinary application cookies pass through; upstream cookies cannot overwrite the random ingress cookie name.
- Explicit Stop and daemon shutdown serialize against session creation, revoke the gateway before stopping the process, and retain endpoint cleanup behavior. Fixture mode does not create browser sessions.

## Verification

- `rtk bash scripts/go.sh test ./... -count=1` — PASS.
- `rtk bash scripts/go.sh build ./cmd/...` — PASS (Linux).
- `rtk bash -lc 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'` — PASS (Windows amd64 cross-build).
- Frontend Vitest — 116/116 PASS; `rtk npm run typecheck` — PASS; `rtk npm run lint` — PASS; `rtk npm run build` — PASS. Vite retains the existing advisory for the 661 kB minified entry chunk.
- Targeted Go tests cover one-time/replayed tickets, cookie and origin boundaries, request and known/chunked response size limits, process ownership, Workbench route scope, desktop tokenless gating, session rejection after shutdown begins, and ingress closure before service stop.
- A headless Chromium smoke ran with the service fixture as a **native Windows process**. It clicked the one-time ticket from another loopback port, ran the first-party bootstrap, rendered the service fixture page, observed `Sec-Fetch-Site: same-origin` on the in-page fetch, and confirmed the final URL did not contain the ticket. Output: `browser_ingress=PASS rendered_page=PASS fetch_site=same-origin ticket_removed=PASS`.
- Reproduction: cross-build `scripts/service_browser_ingress_fixture.go` for Windows with the `service_ingress_fixture` build tag, start the generated `.exe` hidden, run `rtk python scripts/r1-service-browser-ingress-browser-smoke.py`, then stop the fixture PID and remove the generated `.exe`. The fixture-only build tag keeps this helper out of ordinary `go test ./...` and product command builds.
- The first browser attempt used an old WSL fixture still listening on the same forwarded ports and returned 404. We inspected the listener owner, confirmed it was this task's fixture, stopped it, then rebuilt and ran the Windows fixture directly; the native browser smoke passed. The temporary `.exe` and fixture processes were removed/stopped afterward.

## Limits

This verifies the host gateway against a local HTTP fixture, not an AppContainer Node service. The code does not add `internetClientServer`, a Windows inbound firewall rule, WFP filters, or an AppContainer server-side loopback exemption. No actual Node/npm project, browser profile isolation, packaged Workbench, Windows AppContainer listener, WFP state, QQ/MCP/GitHub endpoint, model, business account, or production database was used. The Windows service path remains unqualified until its exact fixed-port listener policy is implemented and validated on an isolated Windows host; the smoke is not stored as a rendered-page JobRun evidence record.
