# R2 CSV input foundation

> Updated: 2026-10-06. Local bounded table representation and Worker range-read code are implemented; R2 support is not release-qualified.

CSV uploads retain their exact submitted bytes and content digest in the immutable MissionInput revision. Intake validates UTF-8 (with an optional BOM), comma-separated quoted records, at most 100,000 total records, 256 fields per record and 64 KiB per field. The parser never evaluates spreadsheet formulas or rewrites cells.

When a Task freezes its input manifest, Worker delivery includes a revision- and digest-bound table summary: delimiter and encoding, header, data-row count, maximum column count, and inferred types sampled from at most the first 100 data rows. The current parser accepts comma-separated CSV and reports that delimiter. Type labels are suggestions; raw source fields remain authoritative. The initial preview contains at most eight rows and 8 KiB of encoded row data. Header names are bounded to 256 bytes each and 8 KiB total in the summary; truncated names remain available through the header range. CSV summaries count toward the existing bounded input context and file limits.

`polis_csv_read_range` requests one exact CSV revision from the current Task manifest. The Kernel requires the active WorkerSession for that Task and checks the caller's manifest digest, input ID, revision and source digest before reading CAS bytes. Data rows are one-based; `start_row: 0` reads the header. Each response is capped at 100 rows and 128 KiB of encoded row data, returns raw strings and a range digest, and writes an append-only event containing the actor/session, exact revision, range and digests without copying cell contents into the event.

The range tool is exposed only on the separately versioned `polis-product-tool-surface@13` zero-egress fake simulation, marked unqualified. The existing real-provider tool surface remains unchanged; it receives the bounded summary but does not yet have the range tool. No real WorkerSession, provider call, database operation or frozen scenario was run for this slice. Historical whole-text CSV delivery receipts remain readable through a legacy reconstruction path.

This code and build evidence do not claim R2 qualification. Owner-approved fixtures, the full format matrix, actual delivery through an authorized active WorkerSession, real provider/harness qualification, multi-day recovery, restore/update checks, external feedback and E-START/E-ORG/E-HANDOVER evidence remain open. Tests were not run.
