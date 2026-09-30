# T11A auth fingerprint reconciliation

## Verified producers

- T8's `scripts/r03a-t8-execution-diff.ps1` calls `Get-FileHash -Algorithm SHA256 -LiteralPath $aAuth` and stores only `auth.sha256`; the label is `user_auth_file_opaque_fingerprint`. B/C point to the same host auth file, read-only mounted as `/home/codex/auth.json`.
- T9's `internal/probe/r03a_t9.go` reads `B.auth.sha256` from the frozen T8 execution diff through `readT8AuthFingerprint`; it does not parse token claims or canonicalize an identity manifest. It writes that value under `auth_identity_fingerprint`.
- T10's qualification state contains `auth_source_class=local_codex_auth_file` inside the combination, but does not record an identity or credential-revision fingerprint; its local qualification did not mount auth or run a provider turn.
- T11's `internal/probe/r03a_t11_runner.go` reads the current auth file bytes and applies the shared raw-byte SHA-256 helper, then compares that value to T9's misnamed field as `AuthIdentityFingerprint`.

## Semantic result

- T9 `775b04c0...` and the T11-observed `3076536b...` are the same algorithm and same raw-file-byte canonicalization: SHA-256 over the complete `auth.json` bytes, with no JSON parsing or normalization. They are comparable as credential-material revisions and are `DIFFERENT`.
- They are not stable account/subject identities. The historical T9/T8 evidence has no claim-level identity digest or claim snapshot, so historical stable identity is `NOT_RECONSTRUCTABLE`; current auth can be parsed locally for a safe stable identity digest from non-secret issuer/subject claims only.
- The T11 comparator treated the raw credential revision as `auth_identity_fingerprint`; this is a fingerprint-type naming/semantic bug. It did not prove that the account changed.
- Source class is the same operational source (`same host auth.json`, read-only mounted for the WSL child); historical combination spelling remains `local_codex_auth_file` and is not rewritten.

## Safe v2 policy

Keep `canonical-manifest-v1` unchanged. Add opt-in `canonical-manifest-v2` with explicit `AuthFingerprintManifest` fields for source class, stable identity schema/value/status, credential revision schema/value/status, and separate stale reason codes. A missing historical identity remains `unavailable`/`NOT_RECONSTRUCTABLE`; a credential revision change must not be interpreted as an identity change.

## T11A decision

`incomparable_fingerprint_types=false` for the two raw-file SHA values, but `identity_comparison=NOT_RECONSTRUCTABLE` and `credential_revision_comparison=DIFFERENT`. The historical T9 strict version-only control is therefore invalid for the current auth material. T11's original `preflight_failed` evidence remains untouched. The next authorized experiment must first establish a current-auth 0.151.0 no-proxy zero-tool baseline; no provider call is permitted in T11A.
