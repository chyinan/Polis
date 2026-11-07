# R0.3A-T17 — Effective CODEX_HOME / Config Differential Qualification

## Result

`T17 = PASSED` as a local configuration qualification only. No Medium, High,
provider egress, or live canary was used. `native_execution_environment` remains
unqualified and `provider_transport_recovery = NOT_YET_VERIFIED`.

## A/B sources

- A is the Windows user profile `/mnt/c/Users/chyinan/.codex`, resolved as the
  working Codex `CODEX_HOME` default. It has a readable `config.toml` and a user
  auth file.
- B is the Polis host-side isolated HOME
  `/mnt/d/Programs/Polis/.runtime/linux/r03a-t14c/home`, exposed to the child as
  guest `HOME=/home/codex` and `CODEX_HOME=/home/codex`. Its readable generated
  `config.toml` is the effective non-secret config file. Its zero-byte host
  `auth.json` is shadowed by the external read-only auth bind and is not the
  effective credential source.

The Windows config contains many unrelated plugin, MCP, shell-environment,
desktop and project-trust settings. Polis B contains generated config plus
runtime-created SQLite/state files. Caches, history, logs, queues, memories,
sessions and state DBs are classified as cache/ephemeral state and are not
treated as effective configuration without direct loading evidence. The full
working `.codex` directory was not copied.

## Confirmed normalized differences

- model: effective `gpt-5.6-luna` on both sides, but A is config-file explicit and
  B is the T14C `thread/start` override.
- reasoning: A `xhigh`; B `medium`.
- sandbox: A `danger-full-access`; B `read-only`.
- A has `network_access=enabled`; B has no equivalent config key (network path
  was independently qualified by T16).
- A has 13 enabled plugin entries and 4 MCP server tables; B has 0/0.
- A has `features.goals=true`; B has `features.goals=false`.
- approval policy is `never` on both sides.
- B explicitly selects `polis-openai`; A has no equivalent explicit
  `model_provider` key.
- A endpoint/base-url shell configuration is present but redacted; B has no
  equivalent explicit endpoint field.

Effective config digests differ: A
`78fe940ce4fb4f562f875eae37d3b8e301040034210c8d2ac4631b99bf5b3de4`, B
`a342c76493b2de2c24f4001f81206678e5b70f6b78a2427b3c40ac6ccc7caf7c`.
Effective transport-config digests differ: A
`fb58b9e3f4ea84e3d261c460ea9f892e437bf552d99ce56241819dff8cc5dee9`, B
`e5e78e023210da320fdf86f3c8504a3c97d535d11fb0a732d6e36229f2a57eb3`.

## Native-default conclusion

`same_native_default = UNPROVEN`. Both sides do not expose the same explicit
WebSocket keys, but omission is not proof of equal default resolution across
Windows vs WSL/Linux, runtime version, and CODEX_HOME. T14C's native-default
therefore cannot be treated as equivalent to the Windows working Codex default.

The direct Windows CLI check confirmed local version `0.153.4`, the
`app-server` subcommand, and config/profile loading options without provider
access, but no complete effective-config dump was available. T14C initialization
evidence confirms B's selected guest `/home/codex` loading boundary only.

## Manifest

Existing V4 could express network namespace policy but not HOME profile or
effective config digests. New `canonical-manifest-v5` records
`codex_home_profile`, `effective_config_digest`, and
`effective_transport_config_digest`; V3/V4 and all historical fingerprints were
not recomputed. The B V5 fingerprint is
`18affdda2307f5ef9154d60bbc510142a684c0d130e022c5fea443169cc24879`.

## Next step

The one next step is a separately authorized config-only minimal live canary:
selected non-secret execution config under isolated HOME, zero dynamic tools,
the tiny sentinel, no business side effects, one Luna Medium, no retry/reset.
Only `first valid output -> turn/completed` may qualify the resulting L1
execution combination.

Evidence: `evidence/development/r0.3a-t17/`. T6/T9/T12/T13/T14C evidence and
all historical raw protocol/allowance records remain unchanged.
