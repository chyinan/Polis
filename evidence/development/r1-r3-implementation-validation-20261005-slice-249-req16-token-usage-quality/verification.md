# Slice 249 — REQ-16 observed token usage quality

## Change

The Codex app-server usage-update count now reaches the Worker terminal observation. When no thread/tokenUsage/updated event was observed, the terminal record contains token_usage=null, token_usage_quality=unavailable, and update count 0. When one or more updates arrive, it records the latest reported usage with quality protocol_reported and the event-source scope. This prevents absent telemetry from being presented as confirmed zero usage.

The Workbench money projection remains unavailable/null. No token-to-currency estimate, ProviderAccount liability, billing mode, or hidden retry count was added. These billing and provider qualification gates remain open.

## Verification

- go build ./... passed on integrated source commit 4530950 (published implementation commit 93b8cb73c1f54dc8a132ff703f820b83ac11e13a).
- git diff --check HEAD^ HEAD passed.
- All REQ-16 scenario rows remain partial/not_run; all 232 frozen scenarios remain not_run.
- No tests, database operation, Worker/provider action, billing/accounting operation, or frozen scenario ran.
