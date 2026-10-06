# Owner policy decisions for remaining gated requirements

The installation owner has selected the following defaults for future enforcement:

- REQ-36 dependency changes: only the existing fixed Node/npm environment policy is in scope; registry hosts remain an explicit allowlist, lifecycle scripts stay disabled, Linux network remains deny-all, Windows network remains registry-only, timeout is at most 10 minutes and output at most 1 MiB. No arbitrary script, new registry, license exception or unbounded package change is autonomous; anything outside this envelope blocks for review.
- REQ-37 BorrowerLease: same-Mission access only, bound to an exact borrower Task and active WorkerSession; one lease per borrower/job generation; maximum TTL 15 minutes; idle grace 2 minutes; owner-session stop, generation change, expiry or borrower stop revokes the lease. Cross-Company, cross-Mission and anonymous borrowers are denied.
- REQ-38 browser/research: default deny; no target URL, identity, cookie, download, WebSocket or control-port access is implicitly allowed. Future policies require explicit HTTPS target origins, a dedicated test identity/data set, isolated profile, redirect/subresource checks and management-network denial before any BrowserRun.
- REQ-40 delivery: ready completion opens a seven-day feedback window; expiry blocks writes without auto-acceptance; Mission success requires explicit accepted delivery dispositions; failed/cancelled closeout does not.

These choices narrow future implementation and do not qualify a host, provider, browser or external account.
