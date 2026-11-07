# R0.5B4 provider diagnostic runtime audit

The current production registry is `provider.ProductToolSurface()` over `codex.ProductEmployeeTools()`, and `cmd/polis` passes that surface to `RealProviderWorkerAdapter`. The B3 raw registration record contains all seven exact provider-visible definitions and the frozen @2 digests.

There is no existing diagnostic-only authorization. `RealProviderWorkerAdapter` creates business Task/session state before calling `Runtime.Reserve`, while `ExecutionAuthorization` requires Company, Mission, Task, Employee and WorkerSession identity. Do not invoke it for this zero-object qualification.

The direct `CodexRuntime` / `Session` API can start the current provider without a database object. Add a separate, purpose-bound diagnostic authorization and atomically created reservation marker (`O_EXCL`); never synthesize business IDs or call the business budget. If a provider attempt starts, the marker stays consumed regardless of subsequent evidence/reporting errors. A diagnostic thread must use an explicit fixed no-tools developer instruction; preserve the existing employee instruction by default.

`runner.NativeLaunchEnvelope` and `DescribeNativeLaunch` already bind OS, launch mode, binary/helper hashes, argv, stdio boundary, and transport policy. Compose a new product-specific execution fingerprint over that envelope, `polis-product-tool-surface@2`, exact manifest/schema identity, and `gpt-5.6-luna/medium`; do not reuse historical B2 or R0.3A fingerprints.

The desktop environment has no `POLIS_PROVIDER_*` values set, but the staged Windows runtime and `.codex/auth.json` are present. The current `codex.exe` and helper hashes were rechecked against the 0.154.0-alpha.6.2 runtime manifest. This confirms local bytes only; it does not qualify or authorize those historical execution fingerprints.
