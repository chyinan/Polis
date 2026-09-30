# R2 CSV input foundation

> Updated: 2026-09-24. Local parser and Worker text path are implemented; R2 support is not release-qualified.

CSV uploads now pass a bounded RFC-4180-style parse before the immutable source is marked usable. The parser accepts UTF-8 with an optional BOM, validates quoted records and embedded newlines, and caps a file at 100,000 records, 256 columns per record, and 64 KiB per field. The original bytes and digest remain authoritative; the Worker receives the original CSV bytes as text inside the existing untrusted-input boundary and frozen manifest receipt.

The Workbench file picker now includes `.csv`. CSV continues to obey the Worker context limits of 16 KiB per source file and 64 KiB aggregate text. A valid upload larger than those limits remains stored but is recorded as excluded from that Worker context.

Offline tests cover valid quoting, embedded newlines, malformed quoted data, and CSV inclusion in the prepared Worker prompt. This CSV slice does not claim database-backed R2 qualification, real provider calls, PDF extraction, or GitHub feedback. ZIP parsing and Worker delivery are documented separately in `R2_ZIP_INPUT_FOUNDATION.md`. R2 still requires its full format matrix, multi-day recovery, external feedback, restore/update checks, and E-START/E-ORG/E-HANDOVER evidence.
