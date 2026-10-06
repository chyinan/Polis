# REQ-37 fake-only read-only JobRun surface

Slice277 implements the smallest Worker-side REQ-37 slice that does not require a real executor, host qualification, or owner-selected BorrowerLease policy.

The isolated `polis-product-tool-surface@15` fake surface adds only `polis_jobs_status` and `polis_jobs_logs`. Both accept one closed `job_id` object and are read-only. Kernel reads use a repeatable-read transaction, revalidate the current WorkerSession, and require the JobRun to match the bound Company, Task, and Session. Log bytes are read only after the same scope check and are verified against the persisted SHA-256 manifest.

The slice intentionally does not add `jobs.start`, `jobs.stop`, process control, service endpoint borrowing, BorrowerLease, or any real-provider qualification. The existing qualified product surface `@4` remains unchanged.
