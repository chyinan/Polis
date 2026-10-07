# Slice 308 — BrowserRun runner and completion boundary

Date: 2026-10-07

## Scope

This slice adds the source-level BrowserRun execution boundary without
opening it in the product runtime. The pure Go policy core validates HTTPS
origin plans, same-origin final URLs, request/response bounds, and rejects
downloads or WebSockets. The imperative runner launches a configured Python
Playwright protocol process with an empty temporary profile, a minimal
environment, bounded stdin/stdout/stderr and no shell. The Python adapter uses
a persistent isolated profile, blocks cross-origin requests and service
workers, rejects downloads, closes WebSockets and emits bounded JSON evidence.

The Kernel now also has a database-bound success fence that accepts only a
current same-Task/Session BrowserRun whose evidence references real ready
candidate/passed Artifact rows. No runner is wired to a Worker/provider path
yet, so the product remains default-deny.

## Verification

```text
./scripts/go.sh test ./internal/browser -count=1
ok   polis/internal/browser  0.045s

./scripts/go.sh test ./internal/kernel -run "(BrowserRun|OperationEvidence)" -count=1
ok   polis/internal/kernel  0.056s

python3 -c "compile(open('scripts/polis_playwright_runner.py', encoding='utf-8').read(), 'scripts/polis_playwright_runner.py', 'exec')"
exit 0
```

The runner shell test uses a disposable local helper, confirms a fresh profile
is removed after execution, and confirms `OPENAI_API_KEY` is not inherited.

## Boundary

No Playwright/Chromium process was launched by this slice, no network request
was sent, and no Worker, Provider, external account, browser qualification
host or frozen scenario was used. Effective profile/WFP/management-network
isolation and product wiring remain qualification gates. The runner is source
available but not enabled by default.

