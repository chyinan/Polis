# Executor and browser qualification checkpoint

The authorized local qualification checked the Windows native runner boundary and the service-browser ingress boundary without starting a business Worker or contacting a real provider.

- WSL Linux `internal/environment` and `internal/runner` tests passed.
- The Windows amd64 runner test binary executed on Windows. AppContainer launch/close, Job Object descendant cleanup, resource limits, loopback/WFP plan guards, registry proxy egress guards, stop proof, outside-file/loopback denial and process-launch classification passed. The isolated verifier test skipped because `POLIS_GO_ROOT` is not configured.
- The service-browser ingress Go tests passed, covering one-time tickets, protected cookies, pinned endpoints, redirects, headers, bounded request/response sizes, methods, concurrency, shutdown and owner checks.
- The Windows browser smoke reached the Workbench fixture but could not reach WSL's randomized `127/8` ingress address (`ERR_CONNECTION_REFUSED`). WSL Playwright Chromium download also could not complete through the available CDN path, and WSL has no passwordless sudo for system Chromium. This is host interop/tooling evidence, not an application isolation pass.

Native browser rendering, WFP effective policy, a prepared Node/npm project, clean-VM packaging and full Windows host qualification remain unqualified.
