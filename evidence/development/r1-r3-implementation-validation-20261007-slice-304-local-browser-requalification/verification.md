# Slice304 local browser requalification checkpoint

Date: 2026-10-07

## Scope

Under the user's local executor/browser qualification authorization, the
repository's `service_browser_ingress_fixture` was started in WSL Ubuntu-22.04
and Windows Chrome was driven headlessly through the installed Chrome channel.
The browser reached the fixed Workbench fixture at `127.0.0.1:45169` and
followed its `Open service` link. The service URL used the randomized WSL
loopback address `127.61.80.131`; Windows Chrome received
`ERR_CONNECTION_REFUSED` at that cross-namespace boundary.

The failure reproduces the prior Slice280 boundary. No URL rewriting,
port-forwarding, firewall change, browser bypass, external URL, Provider,
WorkerSession, account or frozen scenario was used. The WSL fixture process was
stopped and verified absent after the attempt.

## Result

- Windows Chrome → local Workbench fixture: reached.
- Windows Chrome → randomized WSL service ingress: failed with connection refused.
- Service rendering/effective browser isolation qualification: still `not_run`.
- Source changes: none.

The randomized loopback boundary must remain intact; replacing it with a
reachable fixed address would invalidate the isolation claim.
