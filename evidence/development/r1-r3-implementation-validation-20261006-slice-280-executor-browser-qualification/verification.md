# Slice280 verification: executor and browser qualification checkpoint

Date: 2026-10-06

## Executor evidence

- `bash scripts/go.sh test ./internal/environment ./internal/runner -count=1` — passed in WSL.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh test -c ./internal/runner -o .runtime/linux/go-tmp/runner-windows.test.exe` — passed.
- Windows execution of `runner-windows.test.exe -test.v` — passed. The isolated verifier test skipped because `POLIS_GO_ROOT` was not configured; all other runner tests passed, including AppContainer write/loopback denial, Job Object tree cleanup, WFP/registry policy guards, stop proof, service listener ownership, resource limits and process-launch classification.
- `bash scripts/go.sh test ./internal/control -run 'Test(ServiceBrowser|CreateProjectJobBrowserSession)' -count=1` — passed.

## Browser evidence and boundary

- The local service ingress fixture started on WSL and served the Workbench fixture.
- Windows Playwright reached `http://127.0.0.1:45169/`, but the generated randomized WSL ingress URL used a `127/8` address that Windows could not reach; the smoke failed with `ERR_CONNECTION_REFUSED` at that cross-namespace boundary.
- WSL Playwright installation downloaded its Python package but the Chromium CDN download stalled with no browser artifact. System Chromium was unavailable and `sudo` requires a password.

No external URL, provider, business account, WorkerSession, QQ/GitHub/MCP endpoint or frozen scenario was used. Native browser rendering, effective WFP policy, prepared Node/npm execution, clean-VM packaging and cross-day recovery remain `not_run`.
