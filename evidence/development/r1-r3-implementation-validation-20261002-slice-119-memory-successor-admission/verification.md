# Slice 119 — REQ-15 revalidated successor Worker admission

## Change

- Product-provider admission still allows the first WorkerSession as before.
- A later session is admitted only when all prior sessions for the Task are stopped, every prior session has immutable memory revalidation history, and the latest stopped session was revalidated for the current or immediately preceding Task generation.
- The new restoring session must be the latest generation and exactly one generation ahead of the ready Task. The same history and clean-memory gate are rechecked immediately before provider reservation.
- Memory revalidation now advances Task generation for both `ready` and `working` Tasks. Replaying one revalidation cannot authorize repeated successors.
- No migration was needed; the gate reads Schema 77 `memory_task_revalidation_events`, and Kernel startup already requires Schema 80.

## Verification

- `gofmt` on the changed Kernel files: passed.
- `go build ./internal/kernel`: passed.
- `go build ./cmd/polis`: passed.
- `git diff --check`: passed.
- `go build ./cmd/...` encountered an Android/Termux clang tagged-pointer linker crash while linking `cmd/polis-r03a-t14c`. A serial full-command sweep was stopped after it became impractically slow; only the directly affected Kernel and service entry point were built successfully.
- No tests were added or run. No PostgreSQL runtime, Worker process, or provider session was started.

## Boundary

The Workbench memory-correction proposal/review write path still lacks a trustworthy employee-session identity and remains unavailable. Legacy session history without a matching immutable owner revalidation event fails closed. This does not qualify the six frozen REQ-15 scenarios or the restored-backup runtime path.
