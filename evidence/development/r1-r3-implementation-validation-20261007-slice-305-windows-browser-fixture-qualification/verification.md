# Slice305 Windows browser fixture qualification

Date: 2026-10-07

## Scope

With the user's local executor/browser qualification authorization, the
service ingress fixture was cross-compiled for Windows amd64 and run on the
Windows host. Installed Windows Chrome then opened the fixed Workbench fixture,
followed `Open service`, rendered the service page and completed its
same-origin `/meta` fetch.

Observed result:

- Workbench title: `Workbench fixture`.
- Service title: `Polis browser ingress fixture`.
- Rendered body included `rendered; fetch-site=same-origin`.
- The Windows fixture process was stopped and the temporary executable was
  removed.

This fixes the previous WSL namespace mismatch for the local fixture and
qualifies the browser rendering/ingress behavior in the Windows-hosted test
topology. It does not qualify effective WFP policy, isolated browser profile
controls, packaged Node/npm, clean-VM behavior or production Provider paths.
No external URL, account, WorkerSession or frozen scenario was used.
