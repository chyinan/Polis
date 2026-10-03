# Slice 141 — FT-63 owner password hash foundation

Date: 2026-10-03

## Delivered

- Added `internal/installationauth` using the official `golang.org/x/crypto/argon2` implementation, pinned at v0.57.0.
- Hashes use Argon2id with the RFC 9106 second recommended profile (64 MiB, three passes, four lanes), a random 16-byte salt and a 32-byte result. The encoded profile is fixed.
- Verification rejects unsupported/malformed parameters before invoking Argon2 and compares the derived result in constant time. Inputs are bounded to 1 KiB, and a two-operation gate bounds concurrent Argon2 memory use.
- No owner credential, bootstrap code, login session, cookie or Workbench endpoint was added in this slice. FT-63 remains incomplete.

The parameters and Argon2id API follow the [official Go x/crypto Argon2 package documentation](https://pkg.go.dev/golang.org/x/crypto/argon2).

## Verification

| Check | Result |
| --- | --- |
| `go test ./internal/installationauth` | PASS |
| `go build ./internal/installationauth` | PASS |
| `go build ./...` | PASS |
| `git diff --check` | PASS |
| PostgreSQL or HTTP owner-auth flow | Not implemented or run |
