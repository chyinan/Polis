# Slice 322 — Windows profile, firewall and clean-environment qualification

Date: 2026-10-07

## Result boundary

This is a qualification result for the existing REQ-38 implementation. No
source file was changed. `REQ-38` remains `NOT_COMPLETE`.

The strongest available environment was an isolated fresh browser profile on
the existing Windows host. A clean VM, Windows Sandbox, or separate clean
Windows user profile was not available to this task, so this evidence does
not claim `CLEAN_VM = PASS`.

## Environment

```ini
environment_type = existing Windows host + per-run empty Playwright profile
clean_vm = NOT_AVAILABLE
clean_vm_status = PARTIAL_WITH_EXPLICIT_LIMITATION
windows = Windows 11 Pro for Workstations, 10.0.26200, amd64
user = DESKTOP-L1ND50Q\chyinan
source_before = 229289e954ad08dc3500296c50f011749ebf29d1
source_after = 229289e954ad08dc3500296c50f011749ebf29d1
source_change = NONE
```

`WindowsSandbox.exe`, `vmconnect.exe` and `VirtualBoxVM.exe` were not
available. Querying optional Windows virtualization features also required
elevation and was not used as a basis for a clean-VM claim.

## Network profile and WFP/firewall observations

The read-only profile query returned:

| Interface | Windows profile | Connectivity |
| --- | --- | --- |
| WLAN / GZGS-5G 2 | Private | IPv4 Internet |
| Ethernet / 未识别的网络 | Public | IPv4 LocalNetwork |
| Tailscale | Private | IPv4 LocalNetwork |

No `DomainAuthenticated` profile was present. Domain profile availability was
therefore not tested.

All three Windows firewall profiles reported `Enabled=True`. Their
`DefaultInboundAction` and `DefaultOutboundAction` were both
`NotConfigured`; effective policy was not inferred beyond that observation.
No firewall rule or exception was added.

`netsh wfp show state file=-` and `net session` both returned Windows error 5
(`ERROR_ACCESS_DENIED`). The current process is not elevated. Consequently:

```ini
wfp_state = NOT_INSPECTABLE_WITHOUT_ELEVATION
wfp_profile_qualification = NOT_QUALIFIED
firewall_exception_workaround = NONE
```

## Browser/runtime and loopback

The qualification used:

```ini
python = C:\Python314\python.exe (Python 3.14.0)
playwright_package_root = C:\Users\chyinan\AppData\Roaming\Python\Python314\site-packages
browser = C:\Program Files\Google\Chrome\Application\chrome.exe
chrome_version = 155.0.8059.39
runner = scripts/polis_playwright_runner.py
profile = fresh empty per-run temporary directory
```

The Windows amd64 `internal/browser` test binary was cross-compiled in WSL2
and executed natively on Windows with the explicit Python package root and
Chrome path. `TestPlaywrightRunnerWindowsChromeLoopbackTLSFixture` passed in
1.94 seconds. The fixture rendered through Chrome, read same-origin body
text, and produced zero downloads and zero WebSockets. The temporary test
binary and per-run profile were removed. The expected self-signed loopback
TLS handshake warnings were the only server warnings.

```ini
windows_chrome_playwright = PASS
loopback_path = PASS
browser_process_identity = PASS (explicit Chrome executable/version)
profile_isolation = PASS_FOR_PER_RUN_EMPTY_PROFILE
```

This is not a claim that a clean Windows user profile or WFP-isolated VM was
qualified.

## Default-off and explicit local path

The default-off source path remains unchanged: without
`POLIS_OPERATION_ADAPTERS_ENABLED=1`, the product does not inject the
BrowserRun or ResearchOperation adapters. The full Go suite and the existing
default-deny tests passed; no provider credential or external retrieval was
started.

```ini
default_off = PASS (source/configuration and regression evidence)
hidden_network_exposure = NOT_OBSERVED
implicit_browser_start = NOT_OBSERVED
explicit_local_runner_path = PASS (test-only loopback TLS switch)
product_operation_adapter_enable = NOT_RUN (requires real Worker/DB wiring)
```

The explicit local runner path used `TestOnlyAllowInsecureTLS=true`, which is
accepted only for localhost/loopback fixtures. It is not production provider
enablement.

## Failure cases

The real runner boundary and its script were exercised with bounded failure
inputs:

| Case | Result | Partial success evidence |
| --- | --- | --- |
| configured browser executable missing | `state=failed`, `reason_code=browser_plan_invalid` | zero requests/bytes/downloads/WebSockets |
| profile directory non-empty | `state=failed`, `reason_code=browser_plan_invalid` | zero requests/bytes/downloads/WebSockets |
| localhost endpoint unavailable | `state=failed`, `reason_code=browser_execution_failed` | empty final URL/title/text and zero counts; no Chrome process remained |
| missing profile root in Go runner | existing `TestPlaywrightRunnerRejectsMissingProfileRoot` passed | no runner execution |
| WFP/elevation policy restriction | Windows error 5 from WFP inspection | no policy mutation attempted |

All observed browser failures returned structured safe diagnostics and did not
produce a succeeded BrowserRun outcome. No evidence Artifact was emitted by a
failed browser execution.

## Evidence, Workbench and completion fence

These surfaces were not re-created against a clean VM in this run. Their
existing source and disposable PostgreSQL evidence remains passing and was
included in the full Go regression run:

```ini
evidence_artifact_completion_fence = PASS (existing offline/disposable evidence)
workbench_owner_projection = PASS (existing source and frontend regression evidence)
completion_fence = PASS (existing fail-closed regression evidence)
windows_clean_vm_artifact_projection = NOT_RUN
```

The full command `bash scripts/go.sh test ./...` passed, including
`internal/browser`, `internal/control`, `internal/kernel`, `internal/research`
and `internal/workbench`.

## Remaining REQ-38 gates

The following remain explicitly outstanding:

```ini
provider_credential_endpoint_ranking_qualification = OUTSTANDING
real_worker_qualification = OUTSTANDING
external_retrieval_qualification = OUTSTANDING
clean_vm_qualification = OUTSTANDING
wfp_profile_qualification = OUTSTANDING
req38_windows_profile_qualification = PARTIAL_WITH_EXPLICIT_LIMITATION
req38_clean_vm_qualification = PARTIAL_WITH_EXPLICIT_LIMITATION
REQ-38 = NOT_COMPLETE
```
