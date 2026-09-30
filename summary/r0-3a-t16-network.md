# R0.3A-T16 — Sandbox Network Path Differential Qualification

## Result

`T16 = PASSED` as a pure local network-path qualification. No Medium, High,
provider model egress, app-server turn, or internet/DNS probe was used. T14C
remains `INCONCLUSIVE` and unqualified; therefore
`eligible_for_new_L1_live_canary = NO` and T15/L2/Backend were not started.

## Actual launcher and namespace evidence

The T13/T14C launcher derivation produced the actual bwrap network arguments:

```text
--unshare-all
--share-net
```

`--unshare-net` was absent and no other `--unshare-*` flag was present in the
actual argv. Local bubblewrap 0.6.1 help says `--unshare-all` unshares every
supported namespace by default, while `--share-net` retains the caller's
network namespace. The observed effective result confirms that override:

- network namespace: `SAME`
- user namespace: different
- mount namespace: different

Thus the sandbox retains filesystem/process namespace isolation while sharing
the WSL network namespace. The T14C launch binding and capability were checked
against its sealed `execution-manifest.json`; no provider command was started.

## Network differential

Host and guest both exposed categorized `lo` and `eth0` interfaces, with
loopback enabled and a default route via `eth0`. Route relation was `SAME`, and
the guest had a default route. Both had the same private-IPv4 DNS category,
search-domain-present classification, and matching redacted `/proc/net` state
digest. `/etc/resolv.conf` content classification matched; host metadata was
observed as a symlink while the guest bind appeared as a regular file, with no
DNS value written to evidence.

The topology is `shared_wsl_interfaces`, not loopback-only, no-default-route,
or isolated-veth.

## Localhost qualification and T6 interpretation

A single host-side HTTP server bound to `127.0.0.1:<random-port>` and returned
`POLIS_LOCAL_NETWORK_CANARY_OK`. Both WSL host and bwrap guest reached that same
endpoint and received the exact sentinel. Therefore:

```ini
localhost_proxy_reachable_from_guest = YES
bwrap_network_hypothesis = NOT_SUPPORTED
discovered_local_causal_blocker = NO
```

This is endpoint-level localhost reachability only. It does not prove that the
historical T6 proxy instance or provider traffic used that path. T6 evidence was
not rewritten.

## Manifest policy

The existing canonical-manifest-v3 did not explicitly contain a network
namespace factor. New `canonical-manifest-v4` adds
`network_namespace_policy`; this run recorded
`shared_host_network` with fingerprint
`31aadd40f1d198406c403b9c09fc2f50170e2fde0efd09f91f571b56318dfc68`.
The V4 business gate stales an L1 record when this policy changes, while V3 and
all historical fingerprints remain unchanged. Negative tests cover policy
digest change, unknown-policy rejection, and V3 semantic preservation.

Evidence: `evidence/development/r0.3a-t16/`. Early harness-only preflight
failures are preserved as `preflight-attempt-1.json` through
`preflight-attempt-4.json`; intermediate local observations are preserved in
`attempt-1` and `attempt-2`; no historical T12–T14C evidence was changed.
