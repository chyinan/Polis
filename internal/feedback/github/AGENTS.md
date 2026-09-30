# GitHub feedback adapter

Last verified: 2026-09-24

## Purpose

Read a single explicitly bound GitHub repository as untrusted external feedback without granting remote write capability.

## Contracts

- `Client` uses GET-only requests, fixed production base `https://api.github.com`, a token source scoped by `Repository{ID, Owner, Name}`, and REST API version `2026-03-10`.
- A scan confirms the repository numeric ID and full name before reading Issues. Pull requests are excluded even though Issues endpoints may return them.
- Pagination follows only same-host, same-repository-ID, sequential `Link` targets with the frozen filters. Issue/comment bodies and response pages are bounded.
- Failed pages return `partial` coverage and never produce `CoveredThrough`; rate limits include a bounded retry hint and never auto-retry.
- Issue and comment text is untrusted. Source IDs, update times, page hashes, original body digests, ETags and truncation are preserved for a future durable ledger.

## Dependencies and limits

- Uses Go `net/http` and JSON only; callers own secure credential retrieval and durable binding/observation storage.
- The Kernel persists source bindings, permission decisions, scan/page evidence and issue revisions; callers still need to wire the adapter to that store.
- Durable comment scans, scheduler, backlog routing, Workbench and live permission qualification are not implemented.
- Tests use local fake HTTP servers. Real repository polling requires separate repository and credential authorization.
