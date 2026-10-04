# Windows Desktop code signing

## Release build

The checked-in Tauri configuration at `desktop/src-tauri/tauri.signing.conf.json` adds a custom SignTool command only for the explicit signed-release build. From a Windows checkout with frontend dependencies and the packaged PostgreSQL runtime installed, configure `POLIS_WINDOWS_SIGNING_CERT_THUMBPRINT` to the publisher certificate's 40-character SHA-1 thumbprint in the current user's Windows certificate store and `POLIS_WINDOWS_TIMESTAMP_URL` to the certificate issuer's approved timestamp endpoint, then run:

```powershell
.\scripts\build-signed-windows.ps1
```

The wrapper checks the packaged runtime, certificate validity, private-key availability, Code Signing EKU, and Windows SDK SignTool before building. Tauri invokes `scripts/sign-windows.ps1` for each Windows binary it signs; the script uses SHA-256 Authenticode signatures, RFC 3161 timestamping, then verifies the signature with `signtool verify /pa /all`. It exits nonzero when a prerequisite or verification fails. It does not fall back to an unsigned package. Ordinary development builds remain unchanged.

The private key remains in the Windows certificate store and is never stored in this repository. The timestamp URL is an explicit operator setting so the build does not send hashes to an unselected service. This integration requires a publisher-issued certificate; a self-signed development certificate does not qualify a public release.

## Qualification status

This checkout has no valid publisher certificate, Windows SDK host, or packaged Windows PostgreSQL runtime. No signature was created, and the signed build was not run. Certificate identity, timestamp/revocation behavior, resulting installer signatures, first-run behavior, and clean-VM install/update/rollback still require verification on an authorized Windows release host.
