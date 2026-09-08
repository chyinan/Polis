# Current implementation slice

Explicit local test mission → unique bootstrap owned by emp-planning → direct bounded request to emp-backend → evidence-bearing response → fixed candidate → deterministic non-author emp-review verification → process restart recovery. Four fixed profiles include idle emp-frontend. Two scripted FakeWorkers do the collaboration; the checker is trusted deterministic code.

Use scoped internal handles bound by the trusted local CLI, never identity fields in worker payloads. No HTTP worker endpoint or browser authentication is exposed. Keep normalized PG records, company-guard transactions, ordered company events, durable outbox and stable receipts. A controller owns an advisory lock and new incarnation. Only in-process fake work may be recovered automatically; this is not OS writer isolation.

Tests: duplicate/concurrent start and message, body conflict, foreign company access, stale epoch/incarnation and valid neighbor, transaction rollback, pending responsibility and artifact recovery, idle bounded reconciliation, bad/missing/corrupt artifact and author self-approval rejection. Full parent FT/PP scenarios remain not_run when only an R0 variant runs.

No paid worker adapter, workspace command execution, MCP, downloads, GC, QQ, complete UI or business integration in this slice. Go CLI queries are sufficient. Stop after this bounded slice is verified.
